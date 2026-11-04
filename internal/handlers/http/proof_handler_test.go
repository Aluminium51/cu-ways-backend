package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type proofHTTPRepository struct {
	calls  int
	userID int32
	list   []domain.Attachment
	err    error
}

func (f *proofHTTPRepository) CreateForAcceptedMarketer(_ context.Context, marketerID, jobID int32, a *domain.Attachment) error {
	f.calls++
	f.userID = marketerID
	if f.err != nil {
		return f.err
	}
	a.AttachmentID = 41
	a.JobID = jobID
	return nil
}

func (f *proofHTTPRepository) ListForParticipant(_ context.Context, userID, _ int32) ([]domain.Attachment, error) {
	f.calls++
	f.userID = userID
	return f.list, f.err
}

type proofHTTPMembership struct{ marketer bool }

func (f proofHTTPMembership) IsCreator(context.Context, int32) (bool, error)  { return false, nil }
func (f proofHTTPMembership) IsMarketer(context.Context, int32) (bool, error) { return f.marketer, nil }

func proofHandlerApp(repo *proofHTTPRepository, marketer bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "9", Values: map[string]any{}})
		return c.Next()
	})
	h := NewProofHandler(services.NewProofService(repo, proofHTTPMembership{marketer: marketer}))
	app.Post("/jobs/:id/proofs", h.Create)
	app.Get("/jobs/:id/proofs", h.List)
	return app
}

func proofErrorCode(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	res, err := app.Test(httptest.NewRequest(method, path, strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(res.Body).Decode(&envelope)
	return res.StatusCode, envelope.Error.Code
}

func TestProofHandlerCreatesProof(t *testing.T) {
	repo := &proofHTTPRepository{}
	res, err := proofHandlerApp(repo, true).Test(httptest.NewRequest("POST", "/jobs/12/proofs", strings.NewReader(`{"url":"  https://forms.gle/abc123 ","attachment_type":"Proof"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Status string        `json:"status"`
		Data   ProofResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	p := envelope.Data
	if res.StatusCode != 201 || envelope.Status != "success" || repo.userID != 9 || p.AttachmentID != 41 || p.JobID != 12 || p.AttachmentType != domain.AttachmentTypeProof || p.URL != "https://forms.gle/abc123" || p.UploadAt.IsZero() {
		t.Fatalf("unexpected response: %d %+v", res.StatusCode, envelope)
	}
}

func TestProofHandlerRejectsInvalidRequestsWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		marketer         bool
		status           int
		code             string
	}{
		{"bad_job_id", "/jobs/abc/proofs", `{"url":"https://example.com"}`, true, 422, "validation_error"},
		{"zero_job_id", "/jobs/0/proofs", `{"url":"https://example.com"}`, true, 422, "validation_error"},
		{"not_json", "/jobs/12/proofs", `url=https://example.com`, true, 422, "validation_error"},
		{"unknown_field", "/jobs/12/proofs", `{"url":"https://example.com","upload_at":"2026-01-01T00:00:00Z"}`, true, 422, "validation_error"},
		{"two_values", "/jobs/12/proofs", `{"url":"https://example.com"} {}`, true, 422, "validation_error"},
		{"not_marketer", "/jobs/12/proofs", `{"url":"https://example.com"}`, false, 403, "marketer_required"},
		{"unknown_type", "/jobs/12/proofs", `{"url":"https://example.com","attachment_type":"Invoice"}`, true, 422, "invalid_attachment_type"},
		{"brief_type", "/jobs/12/proofs", `{"url":"https://example.com","attachment_type":"Brief"}`, true, 422, "invalid_attachment_type"},
		{"missing_url", "/jobs/12/proofs", `{}`, true, 422, "invalid_proof_url"},
		{"ftp_url", "/jobs/12/proofs", `{"url":"ftp://example.com"}`, true, 422, "invalid_proof_url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &proofHTTPRepository{}
			status, code := proofErrorCode(t, proofHandlerApp(repo, tc.marketer), "POST", tc.path, tc.body)
			if status != tc.status || code != tc.code || repo.calls != 0 {
				t.Fatalf("expected %d %s without a write, got %d %s (calls=%d)", tc.status, tc.code, status, code, repo.calls)
			}
		})
	}
}

func TestProofHandlerMapsRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrJobNotFound, 404, "job_not_found"},
		{domain.ErrJobForbidden, 403, "forbidden"},
		{domain.ErrJobNotInProgress, 409, "job_not_in_progress"},
		{domain.ErrInvalidAttachmentType, 422, "invalid_attachment_type"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			for _, method := range []string{"POST", "GET"} {
				if method == "GET" && tc.err == domain.ErrJobNotInProgress {
					continue
				}
				status, code := proofErrorCode(t, proofHandlerApp(&proofHTTPRepository{err: tc.err}, true), method, "/jobs/12/proofs", `{"url":"https://example.com"}`)
				if status != tc.status || code != tc.code {
					t.Fatalf("%s: expected %d %s, got %d %s", method, tc.status, tc.code, status, code)
				}
			}
		})
	}
}

func TestProofHandlerListsProofs(t *testing.T) {
	newer := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	repo := &proofHTTPRepository{list: []domain.Attachment{
		{AttachmentID: 2, JobID: 12, AttachmentType: domain.AttachmentTypeProof, URL: "https://example.com/b", UploadAt: newer},
		{AttachmentID: 1, JobID: 12, AttachmentType: domain.AttachmentTypeProof, URL: "https://example.com/a", UploadAt: newer.Add(-time.Hour)},
	}}
	res, err := proofHandlerApp(repo, false).Test(httptest.NewRequest("GET", "/jobs/12/proofs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Data []ProofResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || repo.userID != 9 || len(envelope.Data) != 2 || fmt.Sprint(envelope.Data[0].AttachmentID, envelope.Data[1].AttachmentID) != "2 1" {
		t.Fatalf("unexpected list: %d %+v", res.StatusCode, envelope.Data)
	}

	empty, err := proofHandlerApp(&proofHTTPRepository{}, false).Test(httptest.NewRequest("GET", "/jobs/12/proofs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, empty.Body)
	var emptyEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(empty.Body).Decode(&emptyEnvelope); err != nil || string(emptyEnvelope.Data) != "[]" {
		t.Fatalf("an empty list must encode as [], got %s %v", emptyEnvelope.Data, err)
	}
}
