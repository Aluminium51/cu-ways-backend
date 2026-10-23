package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type fakeMarketerJobService struct {
	calls int
	actor services.Actor
	jobID int32
	err   error
}

func (f *fakeMarketerJobService) StartJob(_ context.Context, actor services.Actor, jobID int32) (*ports.StartedJob, error) {
	f.calls++
	f.actor, f.jobID = actor, jobID
	if f.err != nil {
		return nil, f.err
	}
	offerID := int32(31)
	return &ports.StartedJob{
		Job:   domain.Job{JobID: jobID, JobStatus: domain.JobStatusInProgress, AcceptedOfferID: &offerID},
		Offer: domain.Offer{OfferID: offerID, JobID: jobID, UserID: actor.UserID, OfferedPrice: decimal.RequireFromString("9000"), OfferStatus: domain.OfferStatusAccepted},
	}, nil
}

func marketerJobApp(service *fakeMarketerJobService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "9", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/jobs/:id/start", NewMarketerJobHandler(service).Start)
	return app
}

func TestMarketerJobHandlerStartReturnsInProgressJob(t *testing.T) {
	service := &fakeMarketerJobService{}
	res, err := marketerJobApp(service).Test(httptest.NewRequest("POST", "/jobs/12/start", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK || service.jobID != 12 || service.actor.UserID != 9 {
		t.Fatalf("expected 200 for job 12 as marketer 9, got %d job=%d actor=%d", res.StatusCode, service.jobID, service.actor.UserID)
	}
	var envelope struct {
		Data StartJobResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope.Data
	if data.JobID != 12 || data.JobStatus != domain.JobStatusInProgress || data.AcceptedOfferID != 31 ||
		data.Offer.OfferStatus != domain.OfferStatusAccepted || data.Offer.OfferedPrice != "9000.00" || data.Offer.MarketerID != 9 {
		t.Fatalf("unexpected start response: %+v", data)
	}
}

func TestMarketerJobHandlerStartRejectsMalformedJobID(t *testing.T) {
	service := &fakeMarketerJobService{}
	res, err := marketerJobApp(service).Test(httptest.NewRequest("POST", "/jobs/abc/start", nil))
	if err != nil {
		t.Fatal(err)
	}
	closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity || service.calls != 0 {
		t.Fatalf("expected 422 before the service, got %d calls=%d", res.StatusCode, service.calls)
	}
}

func TestMarketerJobHandlerStartMapsErrors(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrMarketerRequired, fiber.StatusForbidden, "marketer_profile_required"},
		{domain.ErrJobForbidden, fiber.StatusForbidden, "job_forbidden"},
		{domain.ErrJobNotFound, fiber.StatusNotFound, "job_not_found"},
		{domain.ErrJobNotAccepted, fiber.StatusConflict, "job_not_accepted"},
		{domain.ErrAcceptedOfferMismatch, fiber.StatusConflict, "accepted_offer_mismatch"},
		{domain.ErrInvalidJob, fiber.StatusUnprocessableEntity, "validation_error"},
		{errors.New("database unavailable"), fiber.StatusInternalServerError, "internal_error"},
	} {
		res, err := marketerJobApp(&fakeMarketerJobService{err: tt.err}).Test(httptest.NewRequest("POST", "/jobs/12/start", nil))
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
