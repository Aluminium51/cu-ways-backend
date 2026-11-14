package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	randv2 "math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	httpapi "github.com/Aluminium51/cu-way-backend/internal/handlers/http"
	"github.com/Aluminium51/cu-way-backend/internal/platform/database"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

type swaggerRequestExamples struct {
	Example  map[string]any `yaml:"example"`
	Examples map[string]struct {
		Value map[string]any `yaml:"value"`
	} `yaml:"examples"`
}

func TestSwaggerWorkflow(t *testing.T) {
	log := zerolog.Nop()
	db := swaggerWorkflowDatabase(t)
	auth := config.AuthConfig{SecretKey: rand.Text(), Algorithm: config.JWTAlgorithm}
	app := New(Dependencies{
		Logger: log, DB: db.GORM(), ReadinessChecker: db, ReadinessTimeout: time.Second,
		DocsPath:       filepath.Join("..", "..", "docs", "openapi.yaml"),
		TokenIssuer:    utils.NewConfiguredJWTIssuer(auth),
		TokenVerifier:  utils.NewConfiguredJWTVerifier(auth),
		PasswordHasher: utils.NewArgon2idPasswordHasher(),
	})

	var spec struct {
		Paths map[string]struct {
			Post struct {
				RequestBody struct {
					Content map[string]swaggerRequestExamples
				} `yaml:"requestBody"`
			}
		}
	}
	if err := yaml.Unmarshal(swaggerRequest(t, app, http.MethodGet, "/docs/openapi.yaml", "", nil, 200), &spec); err != nil {
		t.Fatalf("parse served OpenAPI: %v", err)
	}
	example := func(path, name string) map[string]any {
		t.Helper()
		content := spec.Paths[path].Post.RequestBody.Content["application/json"]
		payload := content.Example
		if name != "" {
			payload = content.Examples[name].Value
		}
		if len(payload) == 0 {
			t.Fatalf("missing POST example %q for %s", name, path)
		}
		return payload
	}
	register := func(role string) httpapi.AuthResponse {
		t.Helper()
		const path = "/api/v1/auth/register"
		body := example(path, role)
		phone, ok := body["phone"].(string)
		if !ok || len(phone) != 10 || phone[0] != '0' || strings.ContainsFunc(phone, func(r rune) bool { return r < '0' || r > '9' }) {
			t.Fatalf("%s example must include a 10-digit phone number as a string", role)
		}
		body["email"] = "swagger-" + strings.ToLower(rand.Text()) + "@example.test"
		body["phone"] = fmt.Sprintf("09%08d", randv2.IntN(100_000_000))
		account := swaggerData[httpapi.AuthResponse](t, app, http.MethodPost, path, "", body, 201)
		if account.AccessToken == "" || account.User.UserID < 1 {
			t.Fatalf("%s registration did not return an account and token", role)
		}
		return account
	}

	creator := register("creator")
	marketer := register("marketer")
	const loginPath = "/api/v1/auth/login"
	login := example(loginPath, "")
	login["email"] = creator.User.Email
	creator = swaggerData[httpapi.AuthResponse](t, app, http.MethodPost, loginPath, "", login, 200)

	const surveyPath = "/api/v1/surveys"
	survey := swaggerData[httpapi.SurveyResponse](t, app, http.MethodPost, surveyPath, creator.AccessToken, example(surveyPath, ""), 201)
	const jobPath = "/api/v1/jobs"
	jobBody := example(jobPath, "")
	jobBody["survey_ids"] = []int32{survey.SurveyID}
	job := swaggerData[httpapi.JobResponse](t, app, http.MethodPost, jobPath, creator.AccessToken, jobBody, 201)
	jobURL := fmt.Sprintf("%s/%d", jobPath, job.JobID)
	swaggerRequest(t, app, http.MethodGet, "/api/v1/admin/jobs", creator.AccessToken, nil, 403)
	swaggerRequest(t, app, http.MethodPost, jobURL+"/requests", creator.AccessToken,
		map[string]any{"marketer_ids": []int32{creator.User.UserID}}, 409)

	swaggerData[httpapi.BriefResponse](t, app, http.MethodPost, jobURL+"/briefs", creator.AccessToken, example(jobPath+"/{id}/briefs", ""), 201)
	invite := example(jobPath+"/{id}/requests", "")
	invite["marketer_ids"] = []int32{marketer.User.UserID}
	invitations := swaggerData[[]httpapi.JobRequestResponse](t, app, http.MethodPost, jobURL+"/requests", creator.AccessToken, invite, 201)
	if len(invitations) != 1 {
		t.Fatalf("expected one invitation, got %d", len(invitations))
	}
	requestURL := fmt.Sprintf("/api/v1/me/job-requests/%d", invitations[0].JobRequestID)
	swaggerData[httpapi.JobRequestResponse](t, app, http.MethodPost, requestURL+"/accept", marketer.AccessToken, nil, 200)
	offer := swaggerData[httpapi.OfferResponse](t, app, http.MethodPost, requestURL+"/offers", marketer.AccessToken, example("/api/v1/me/job-requests/{id}/offers", ""), 201)
	swaggerData[httpapi.AcceptOfferResponse](t, app, http.MethodPost, fmt.Sprintf("%s/offers/%d/accept", jobURL, offer.OfferID), creator.AccessToken, nil, 200)
	marketerView := swaggerData[httpapi.JobSettlementResponse](t, app, http.MethodGet, jobURL+"/payments", marketer.AccessToken, nil, 200)
	if marketerView.Contact == nil || marketerView.Contact.Email != creator.User.Email {
		t.Fatal("expected the hired marketer to see the creator's contact")
	}
	swaggerRequest(t, app, http.MethodPatch, fmt.Sprintf("%s/%d", surveyPath, survey.SurveyID),
		creator.AccessToken, map[string]any{"title": "Changed after hiring"}, 409)
	swaggerRequest(t, app, http.MethodPost, jobURL+"/briefs", creator.AccessToken, map[string]any{"url": "https://example.test/late-brief"}, 409)
	swaggerData[httpapi.StartJobResponse](t, app, http.MethodPost, jobURL+"/start", marketer.AccessToken, nil, 200)

	payment := swaggerData[httpapi.PaymentResponse](t, app, http.MethodPost, jobURL+"/payments", creator.AccessToken, example(jobPath+"/{id}/payments", ""), 201)
	if payment.PaymentStatus != "Paid" || payment.PaidAt == nil || payment.Amount != offer.OfferedPrice {
		t.Fatal("expected the accepted price to be recorded as Paid")
	}
	conflict := swaggerRequest(t, app, http.MethodPost, jobURL+"/complete", creator.AccessToken, nil, 409)
	if !bytes.Contains(conflict, []byte(`"completion_proof_required"`)) {
		t.Fatal("expected completion to require proof even after payment")
	}
	swaggerData[httpapi.ProofResponse](t, app, http.MethodPost, jobURL+"/proofs", marketer.AccessToken, example(jobPath+"/{id}/proofs", ""), 201)
	swaggerRequest(t, app, http.MethodPost, jobURL+"/complete", creator.AccessToken, map[string]any{"accepted_offer_id": offer.OfferID + 1}, 422)
	completed := swaggerData[httpapi.JobSettlementResponse](t, app, http.MethodPost, jobURL+"/complete", creator.AccessToken, nil, 200)
	if completed.JobStatus != "Completed" || completed.CompletedAt == nil || completed.PayoutStatus == nil || *completed.PayoutStatus != "Released" {
		t.Fatal("expected completed job with recorded simulated payout")
	}
	repeated := swaggerData[httpapi.JobSettlementResponse](t, app, http.MethodPost, jobURL+"/complete", creator.AccessToken, nil, 200)
	if repeated.JobStatus != "Completed" || repeated.CompletedAt == nil || !repeated.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatal("expected repeated completion to return the original completion time")
	}

	review := swaggerData[httpapi.ReviewResponse](t, app, http.MethodPost, jobURL+"/reviews", creator.AccessToken, example(jobPath+"/{id}/reviews", ""), 201)
	saved := swaggerData[httpapi.CurrentReviewResponse](t, app, http.MethodGet, jobURL+"/reviews/current", creator.AccessToken, nil, 200)
	if saved.Review == nil || saved.Review.ReviewID != review.ReviewID || saved.Review.Rating != 5 {
		t.Fatal("expected saved review to be returned on readback")
	}
	// Registration grants Creator membership. Remove it to exercise owner access.
	if err := db.GORM().Exec("DELETE FROM creators WHERE user_id = ?", marketer.User.UserID).Error; err != nil {
		t.Fatalf("remove the marketer's Creator membership: %v", err)
	}
	profileReviewsURL := fmt.Sprintf("/api/v1/marketers/%d/reviews", marketer.User.UserID)
	profileReviews := swaggerData[httpapi.MarketerReviewsResponse](t, app, http.MethodGet, profileReviewsURL, marketer.AccessToken, nil, 200)
	if len(profileReviews.Items) != 1 || profileReviews.Items[0].ReviewID != review.ReviewID ||
		profileReviews.Items[0].ReviewerName != creator.User.Name {
		t.Fatalf("expected the marketer to see the saved review with the Creator's name, got %+v", profileReviews.Items)
	}
	history := swaggerData[httpapi.TransactionListResponse](t, app, http.MethodGet, "/api/v1/me/transactions?role=creator", creator.AccessToken, nil, 200)
	if len(history.Items) != 1 || history.Items[0].JobID != job.JobID || history.Items[0].AcceptedPrice != offer.OfferedPrice ||
		history.Items[0].HasReview == nil || !*history.Items[0].HasReview {
		t.Fatal("expected the completed and reviewed job in the Creator's transactions")
	}

	if err := db.GORM().Exec("UPDATE users SET deleted_at = NOW() WHERE user_id = ?", marketer.User.UserID).Error; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{jobURL + "/payments", jobURL + "/proofs", "/api/v1/me/transactions?role=marketer"} {
		swaggerRequest(t, app, http.MethodGet, path, marketer.AccessToken, nil, 401)
	}
}

func swaggerWorkflowDatabase(t *testing.T) *database.DB {
	t.Helper()
	baseURL := swaggerTestDatabaseURL(t)
	open := func(connectionURL string) *database.DB {
		t.Helper()
		db, err := database.New(config.DatabaseConfig{URL: connectionURL, MaxOpenConns: 4, MaxIdleConns: 2}, zerolog.Nop(), "test")
		if err != nil {
			t.Fatal("could not connect to the Swagger workflow test database")
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error("could not close the Swagger workflow test database")
			}
		})
		return db
	}
	base := open(baseURL.String())
	// Registration opens its own transaction. Isolate its committed records
	// from the repository tests that use the same database.
	schema := "swagger_" + strings.ToLower(rand.Text())
	if err := base.GORM().Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal("could not create the isolated Swagger workflow schema")
	}
	t.Cleanup(func() {
		if err := base.GORM().Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("could not drop the isolated Swagger workflow schema")
		}
	})
	query := baseURL.Query()
	query.Set("search_path", schema)
	baseURL.RawQuery = query.Encode()
	scopedURL := baseURL.String()
	runner, err := migrate.New("file://../../migrations", scopedURL)
	if err != nil {
		t.Fatal("could not initialize migrations for the Swagger workflow schema")
	}
	upErr := runner.Up()
	sourceErr, databaseErr := runner.Close()
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		t.Fatalf("could not apply Swagger workflow migrations (%T)", upErr)
	}
	if sourceErr != nil || databaseErr != nil {
		t.Fatal("could not close the Swagger workflow migration runner")
	}
	return open(scopedURL)
}

func swaggerTestDatabaseURL(t *testing.T) *url.URL {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable loopback database to run the Swagger workflow")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	host, db := u.Hostname(), strings.TrimPrefix(u.Path, "/")
	if (host != "localhost" && !net.ParseIP(host).IsLoopback()) || !strings.HasSuffix(db, "_test") || strings.Contains(db, "/") {
		t.Fatal("TEST_DATABASE_URL must target a loopback database ending in _test")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL query")
	}
	for key := range query {
		if key != "sslmode" && key != "connect_timeout" {
			t.Fatal("TEST_DATABASE_URL supports only sslmode and connect_timeout query parameters")
		}
	}
	return u
}

func swaggerRequest(t *testing.T, app *fiber.App, method, path, token string, body map[string]any, want int) []byte {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, 10_000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer closeResponseBody(t, res.Body)
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	if res.StatusCode != want {
		var envelope struct{ Error struct{ Code string } }
		_ = json.Unmarshal(data, &envelope)
		t.Fatalf("%s %s returned %d, want %d (error code %q)", method, path, res.StatusCode, want, envelope.Error.Code)
	}
	return data
}

func swaggerData[T any](t *testing.T, app *fiber.App, method, path, token string, body map[string]any, want int) T {
	t.Helper()
	var envelope struct {
		Status string
		Data   T
	}
	data := swaggerRequest(t, app, method, path, token, body, want)
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Status != "success" {
		t.Fatalf("invalid success envelope for %s %s", method, path)
	}
	return envelope.Data
}
