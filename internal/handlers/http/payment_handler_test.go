package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type paymentHTTPRepository struct {
	calls     int
	creatorID int32
	err       error
}

func (f *paymentHTTPRepository) CreateForCreatorJob(_ context.Context, creator, job int32, p *domain.Payment) (*domain.Payment, error) {
	f.calls++
	f.creatorID = creator
	if f.err != nil {
		return nil, f.err
	}
	p.PaymentID = 31
	p.JobID = job
	return p, nil
}

type paymentHTTPMembership struct{ creator bool }

func (f paymentHTTPMembership) IsCreator(context.Context, int32) (bool, error)  { return f.creator, nil }
func (f paymentHTTPMembership) IsMarketer(context.Context, int32) (bool, error) { return false, nil }

func paymentHandlerApp(repo *paymentHTTPRepository, authenticated, creator bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if authenticated {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
			return c.Next()
		})
	}
	h := NewPaymentHandler(services.NewPaymentService(repo, paymentHTTPMembership{creator: creator}))
	app.Post("/jobs/:id/payments", h.Create)
	return app
}

func TestPaymentHandlerCreatesPayment(t *testing.T) {
	repo := &paymentHTTPRepository{}
	res, err := paymentHandlerApp(repo, true, true).Test(httptest.NewRequest("POST", "/jobs/12/payments", strings.NewReader(`{"method":"PromptPay","amount":"1500"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Status string          `json:"status"`
		Data   PaymentResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	p := envelope.Data
	if res.StatusCode != 201 || envelope.Status != "success" || repo.creatorID != 8 || p.PaymentID != 31 || p.JobID != 12 || p.Amount != "1500.00" || p.Method != domain.PaymentMethodPromptPay || p.PaymentStatus != domain.PaymentStatusPaid || p.CreatedAt == nil || p.PaidAt == nil || !p.PaidAt.Equal(*p.CreatedAt) {
		t.Fatalf("unexpected response: %d %+v", res.StatusCode, envelope)
	}
}

func TestPaymentHandlerRejectsInvalidPayload(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"method":"Other","amount":"100"}`, `{"method":"promptpay","amount":"100"}`, `{"method":"PromptPay","amount":100}`, `{"method":"PromptPay","amount":"-1"}`, `{"method":"PromptPay","amount":"1.001"}`, `{"method":"PromptPay","amount":"100","payment_status":"Paid"}`, `{"method":"PromptPay","amount":"100","paid_at":"2026-10-29"}`, `{"method":"PromptPay","amount":"100","created_at":"2026-10-29"}`, `{"method":"PromptPay","amount":"100","creator_id":8}`, `{"method":"PromptPay","amount":"100"} {}`} {
		repo := &paymentHTTPRepository{}
		res, err := paymentHandlerApp(repo, true, true).Test(httptest.NewRequest("POST", "/jobs/12/payments", strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		defer closeResponseBody(t, res.Body)
		if res.StatusCode != 422 || repo.calls != 0 {
			t.Fatalf("%s: status=%d calls=%d", body, res.StatusCode, repo.calls)
		}
	}
}

func TestPaymentHandlerMapsErrors(t *testing.T) {
	for _, tc := range []struct {
		name          string
		auth, creator bool
		id            string
		err           error
		status        int
		code          string
	}{
		{"unauthorized", false, true, "12", nil, 401, "unauthorized"},
		{"not_creator", true, false, "12", nil, 403, "creator_required"},
		{"invalid_id", true, true, "0", nil, 422, "validation_error"},
		{"foreign", true, true, "12", domain.ErrJobForbidden, 403, "forbidden"},
		{"missing", true, true, "12", domain.ErrJobNotFound, 404, "job_not_found"},
		{"not_payable", true, true, "12", domain.ErrJobNotPayable, 409, "job_not_payable"},
		{"no_offer", true, true, "12", domain.ErrAcceptedOfferRequired, 409, "accepted_offer_required"},
		{"duplicate", true, true, "12", domain.ErrPaymentAlreadyExists, 409, "payment_already_exists"},
		{"mismatch", true, true, "12", domain.ErrPaymentAmountMismatch, 422, "payment_amount_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &paymentHTTPRepository{err: tc.err}
			res, err := paymentHandlerApp(repo, tc.auth, tc.creator).Test(httptest.NewRequest("POST", "/jobs/"+tc.id+"/payments", strings.NewReader(`{"method":"PromptPay","amount":"100"}`)))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.status || envelope.Error.Code != tc.code {
				t.Fatalf("got %d/%s, want %d/%s", res.StatusCode, envelope.Error.Code, tc.status, tc.code)
			}
		})
	}
}
