package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type fakeJobOfferService struct {
	items     []ports.JobOfferComparison
	status    *domain.OfferStatus
	listCalls int
	err       error
	acceptJob int32
	acceptID  int32
}

func (f *fakeJobOfferService) AcceptOffer(_ context.Context, _ services.Actor, jobID, offerID int32) (*ports.OfferAcceptance, error) {
	f.acceptJob = jobID
	f.acceptID = offerID
	if f.err != nil {
		return nil, f.err
	}
	requestID := int32(21)
	return &ports.OfferAcceptance{
		Job:                 domain.Job{JobID: jobID, JobStatus: domain.JobStatusAccepted, AcceptedOfferID: &offerID},
		Offer:               domain.Offer{OfferID: offerID, JobID: jobID, UserID: 9, JobRequestID: &requestID, OfferedPrice: decimal.RequireFromString("9000"), OfferStatus: domain.OfferStatusAccepted},
		RejectedOfferIDs:    []int32{32},
		ClosedJobRequestIDs: []int32{22, 23},
	}, nil
}

func (f *fakeJobOfferService) ListOffers(_ context.Context, _ services.Actor, _ int32, status *domain.OfferStatus) ([]ports.JobOfferComparison, error) {
	f.listCalls++
	f.status = status
	return f.items, f.err
}

func jobOfferApp(service *fakeJobOfferService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
		return c.Next()
	})
	handler := NewJobOfferHandler(service)
	app.Get("/jobs/:id/offers", handler.List)
	app.Post("/jobs/:id/offers/:offerId/accept", handler.Accept)
	return app
}

func TestJobOfferHandlerReturnsOffersWithProfileAndRating(t *testing.T) {
	average := 4.5
	requestID := int32(21)
	delivery := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	service := &fakeJobOfferService{items: []ports.JobOfferComparison{
		{
			Offer:    domain.Offer{OfferID: 31, JobID: 12, UserID: 9, JobRequestID: &requestID, OfferedPrice: decimal.RequireFromString("9000"), DeliveryDate: &delivery, OfferStatus: domain.OfferStatusPending},
			Marketer: domain.Marketer{UserID: 9, Bio: "Campus reach", ExperienceYears: 3, User: &domain.User{UserID: 9, Name: "Rated Marketer"}, Expertise: []domain.Expertise{{Slug: "social-media", Name: "Social media"}}},
			Rating:   ports.MarketerRating{AverageRating: &average, ReviewCount: 2, CompletedJobs: 3},
		},
		{
			Offer:    domain.Offer{OfferID: 32, JobID: 12, UserID: 10, OfferedPrice: decimal.RequireFromString("7500.5"), OfferStatus: domain.OfferStatusPending},
			Marketer: domain.Marketer{UserID: 10, User: &domain.User{UserID: 10, Name: "New Marketer"}},
		},
	}}
	res, err := jobOfferApp(service).Test(httptest.NewRequest("GET", "/jobs/12/offers?status=Pending", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK || service.status == nil || *service.status != domain.OfferStatusPending {
		t.Fatalf("expected 200 with status filter, got %d %v", res.StatusCode, service.status)
	}
	var envelope struct {
		Data []JobOfferComparisonResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 2 {
		t.Fatalf("expected two offers, got %d", len(envelope.Data))
	}
	rated, unrated := envelope.Data[0], envelope.Data[1]
	if rated.OfferID != 31 || rated.OfferedPrice != "9000.00" || rated.Marketer.Name != "Rated Marketer" || len(rated.Marketer.Expertise) != 1 {
		t.Fatalf("unexpected rated offer: %+v", rated)
	}
	if !rated.Rating.IsRated || rated.Rating.AverageRating == nil || *rated.Rating.AverageRating != 4.5 || rated.Rating.CompletedJobs != 3 {
		t.Fatalf("unexpected rating: %+v", rated.Rating)
	}
	if unrated.Rating.IsRated || unrated.Rating.AverageRating != nil || unrated.OfferedPrice != "7500.50" || unrated.Marketer.Expertise == nil {
		t.Fatalf("unrated marketer must report is_rated=false with empty lists, got %+v", unrated)
	}
}

func TestJobOfferHandlerRejectsUnknownStatusAndMapsErrors(t *testing.T) {
	service := &fakeJobOfferService{}
	res, err := jobOfferApp(service).Test(httptest.NewRequest("GET", "/jobs/12/offers?status=Archived", nil))
	if err != nil {
		t.Fatal(err)
	}
	closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity || service.listCalls != 0 {
		t.Fatalf("expected 422 before service, got %d calls=%d", res.StatusCode, service.listCalls)
	}

	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrCreatorRequired, fiber.StatusForbidden, "creator_required"},
		{domain.ErrJobForbidden, fiber.StatusForbidden, "forbidden"},
		{domain.ErrJobNotFound, fiber.StatusNotFound, "job_not_found"},
	} {
		res, err := jobOfferApp(&fakeJobOfferService{err: tt.err}).Test(httptest.NewRequest("GET", "/jobs/12/offers", nil))
		if err != nil {
			t.Fatal(err)
		}
		var envelope response.ErrorEnvelope
		decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
		closeResponseBody(t, res.Body)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if res.StatusCode != tt.status || envelope.Error.Code != tt.code {
			t.Fatalf("%v: expected %d/%s, got %d/%s", tt.err, tt.status, tt.code, res.StatusCode, envelope.Error.Code)
		}
	}
}

func TestJobOfferHandlerAcceptReturnsResolvedRows(t *testing.T) {
	service := &fakeJobOfferService{}
	res, err := jobOfferApp(service).Test(httptest.NewRequest("POST", "/jobs/12/offers/31/accept", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK || service.acceptJob != 12 || service.acceptID != 31 {
		t.Fatalf("expected 200 for job 12 offer 31, got %d job=%d offer=%d", res.StatusCode, service.acceptJob, service.acceptID)
	}
	var envelope struct {
		Data AcceptOfferResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope.Data
	if data.JobStatus != domain.JobStatusAccepted || data.AcceptedOfferID != 31 || data.Offer.OfferStatus != domain.OfferStatusAccepted || data.Offer.OfferedPrice != "9000.00" {
		t.Fatalf("unexpected acceptance: %+v", data)
	}
	if len(data.RejectedOfferIDs) != 1 || len(data.ClosedJobRequestIDs) != 2 {
		t.Fatalf("expected resolved row IDs, got %+v", data)
	}
}

func TestJobOfferHandlerAcceptMapsTransitionErrors(t *testing.T) {
	res, err := jobOfferApp(&fakeJobOfferService{}).Test(httptest.NewRequest("POST", "/jobs/12/offers/abc/accept", nil))
	if err != nil {
		t.Fatal(err)
	}
	closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a malformed offer ID, got %d", res.StatusCode)
	}
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrOfferNotFound, fiber.StatusNotFound, "offer_not_found"},
		{domain.ErrJobNotPending, fiber.StatusConflict, "job_not_pending"},
		{domain.ErrOfferNotPending, fiber.StatusConflict, "offer_not_pending"},
		{domain.ErrJobForbidden, fiber.StatusForbidden, "forbidden"},
	} {
		res, err := jobOfferApp(&fakeJobOfferService{err: tt.err}).Test(httptest.NewRequest("POST", "/jobs/12/offers/31/accept", nil))
		if err != nil {
			t.Fatal(err)
		}
		var envelope response.ErrorEnvelope
		decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
		closeResponseBody(t, res.Body)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if res.StatusCode != tt.status || envelope.Error.Code != tt.code {
			t.Fatalf("%v: expected %d/%s, got %d/%s", tt.err, tt.status, tt.code, res.StatusCode, envelope.Error.Code)
		}
	}
}
