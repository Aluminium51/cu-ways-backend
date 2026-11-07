package httpapi

import (
	"context"
	"encoding/json"
	"io"
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
	"github.com/shopspring/decimal"
)

type paymentHTTPRepository struct {
	calls     int
	creatorID int32
	jobID     int32
	err       error
	active    *domain.Payment
}

func (f *paymentHTTPRepository) FindActiveForCreatorJob(_ context.Context, creator, job int32) (*domain.Payment, error) {
	f.calls++
	f.creatorID = creator
	f.jobID = job
	if f.err != nil {
		return nil, f.err
	}
	return f.active, nil
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

func paymentHandlerApp(repo *paymentHTTPRepository, authenticated, creator bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if authenticated {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
			return c.Next()
		})
	}
	h := NewPaymentHandler(services.NewPaymentService(repo, fakeMembership{creator: creator}))
	app.Post("/jobs/:id/payments", h.Create)
	app.Get("/jobs/:id/payments/current", h.Current)
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

func getCurrentPayment(t *testing.T, app *fiber.App, id string) (int, string, *PaymentResponse) {
	t.Helper()
	res, err := app.Test(httptest.NewRequest("GET", "/jobs/"+id+"/payments/current", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Status string                 `json:"status"`
		Data   CurrentPaymentResponse `json:"data"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, envelope.Error.Code, envelope.Data.Payment
}

func TestPaymentHandlerReturnsCurrentPayment(t *testing.T) {
	createdAt := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		active *domain.Payment
	}{
		{"paid", &domain.Payment{PaymentID: 4, JobID: 12, Method: domain.PaymentMethodBankTransfer, Amount: decimal.RequireFromString("2500.5"), PaymentStatus: domain.PaymentStatusPaid, CreatedAt: &createdAt, PaidAt: &createdAt}},
		{"pending_legacy", &domain.Payment{PaymentID: 5, JobID: 12, Method: domain.PaymentMethodOther, Amount: decimal.RequireFromString("10"), PaymentStatus: domain.PaymentStatusPending}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &paymentHTTPRepository{active: tc.active}
			status, _, p := getCurrentPayment(t, paymentHandlerApp(repo, true, true), "12")
			want := tc.active
			if status != 200 || repo.creatorID != 8 || repo.jobID != 12 || p == nil || p.PaymentID != want.PaymentID || p.Method != want.Method || p.Amount != want.Amount.StringFixed(2) || p.PaymentStatus != want.PaymentStatus || (want.PaidAt == nil) != (p.PaidAt == nil) || (want.CreatedAt == nil) != (p.CreatedAt == nil) {
				t.Fatalf("unexpected current payment: %d %+v", status, p)
			}
		})
	}
}

func TestPaymentHandlerReturnsNullWithoutActivePayment(t *testing.T) {
	app := paymentHandlerApp(&paymentHTTPRepository{}, true, true)
	res, err := app.Test(httptest.NewRequest("GET", "/jobs/12/payments/current", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || string(body) != `{"status":"success","data":{"payment":null}}` {
		t.Fatalf("unexpected response: %d %s", res.StatusCode, body)
	}
}

func TestPaymentHandlerCurrentMapsErrors(t *testing.T) {
	for _, tc := range []struct {
		name          string
		auth, creator bool
		id            string
		err           error
		status        int
		code          string
		calls         int
	}{
		{"unauthorized", false, true, "12", nil, 401, "unauthorized", 0},
		{"not_creator", true, false, "12", nil, 403, "creator_required", 0},
		{"invalid_id", true, true, "0", nil, 422, "validation_error", 0},
		{"non_numeric_id", true, true, "abc", nil, 422, "validation_error", 0},
		{"foreign", true, true, "12", domain.ErrJobForbidden, 403, "forbidden", 1},
		{"missing", true, true, "12", domain.ErrJobNotFound, 404, "job_not_found", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &paymentHTTPRepository{err: tc.err}
			status, code, _ := getCurrentPayment(t, paymentHandlerApp(repo, tc.auth, tc.creator), tc.id)
			if status != tc.status || code != tc.code || repo.calls != tc.calls {
				t.Fatalf("got %d/%s calls=%d, want %d/%s calls=%d", status, code, repo.calls, tc.status, tc.code, tc.calls)
			}
		})
	}
}
