package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type fakeJobSettlementService struct {
	record *domain.JobSettlement
	err    error
}

func (f fakeJobSettlementService) GetPayment(context.Context, services.Actor, int32) (*domain.JobSettlement, error) {
	return f.record, f.err
}

func (f fakeJobSettlementService) Complete(context.Context, services.Actor, int32) (*domain.JobSettlement, error) {
	return f.record, f.err
}

func jobSettlementHandlerApp(service fakeJobSettlementService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8"})
		return c.Next()
	})
	handler := NewJobSettlementHandler(service)
	app.Get("/jobs/:id/payments", handler.GetPayment)
	app.Post("/jobs/:id/complete", handler.Complete)
	return app
}

func TestJobSettlementHandlerMapsErrors(t *testing.T) {
	for _, tt := range []struct {
		method, path  string
		err           error
		status        int
		code, message string
	}{
		{"POST", "/jobs/12/complete", domain.ErrJobNotInProgress, 409, "job_not_in_progress", "only in-progress jobs can be completed"},
		{"POST", "/jobs/12/complete", domain.ErrCompletionPaymentRequired, 409, "completion_payment_required", domain.ErrCompletionPaymentRequired.Error()},
		{"POST", "/jobs/12/complete", domain.ErrCompletionProofRequired, 409, "completion_proof_required", domain.ErrCompletionProofRequired.Error()},
		{"GET", "/jobs/12/payments", domain.ErrJobForbidden, 403, "forbidden", "you do not have access to this job"},
	} {
		t.Run(tt.code, func(t *testing.T) {
			app := jobSettlementHandlerApp(fakeJobSettlementService{err: tt.err})
			res, err := app.Test(httptest.NewRequest(tt.method, tt.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			var envelope struct {
				Error struct {
					Code    string
					Message string
				}
			}
			if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tt.status || envelope.Error.Code != tt.code || envelope.Error.Message != tt.message {
				t.Fatalf("got status %d and error %+v", res.StatusCode, envelope.Error)
			}
		})
	}
}

func TestJobSettlementHandlerReturnsNullPayoutStatusForCancelledJob(t *testing.T) {
	record := &domain.JobSettlement{
		JobID: 12, JobStatus: domain.JobStatusCancelled,
		Payment: &domain.Payment{PaymentID: 10, PaymentStatus: domain.PaymentStatusPaid},
	}
	app := jobSettlementHandlerApp(fakeJobSettlementService{record: record})
	res, err := app.Test(httptest.NewRequest("GET", "/jobs/12/payments", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Data map[string]json.RawMessage
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || string(envelope.Data["payout_status"]) != "null" || len(envelope.Data["payment"]) == 0 || string(envelope.Data["payment"]) == "null" {
		t.Fatalf("unexpected payment response: status %d, data %+v", res.StatusCode, envelope.Data)
	}
}
