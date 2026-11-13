package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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

type fakeTransactionService struct {
	page   ports.TransactionPage
	err    error
	actor  services.Actor
	query  ports.TransactionQuery
	called bool
}

func (f *fakeTransactionService) List(_ context.Context, actor services.Actor, query ports.TransactionQuery) (ports.TransactionPage, error) {
	f.called = true
	f.actor = actor
	f.query = query
	return f.page, f.err
}

func transactionHandlerApp(service transactionService, authenticated bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if authenticated {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "17", Values: map[string]any{}})
			return c.Next()
		})
	}
	app.Get("/transactions", NewTransactionHandler(service).List)
	return app
}

func TestTransactionHandlerReturnsBreakdownEnvelope(t *testing.T) {
	paidAt := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	service := &fakeTransactionService{page: ports.TransactionPage{
		Page: 2, PageSize: 5, Total: 7,
		Items: []domain.Transaction{{
			JobID: 42, JobTitle: "Campus survey", CreatorID: 17, CreatorName: "Alex Creator",
			MarketerID: 23, MarketerName: "Mali Marketer", AcceptedPrice: decimal.RequireFromString("2500"),
			PaymentMethod: domain.PaymentMethodPromptPay, PaidAt: &paidAt, PayoutStatus: domain.PayoutStatusReleased,
		}},
	}}
	app := transactionHandlerApp(service, true)

	res, err := app.Test(httptest.NewRequest("GET", "/transactions?role=creator&keyword=campus&page=2&page_size=5", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Status string                  `json:"status"`
		Data   TransactionListResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if service.actor.UserID != 17 || service.query.Role != domain.TransactionRoleCreator || service.query.Keyword != "campus" || service.query.Page != 2 || service.query.PageSize != 5 {
		t.Fatalf("unexpected service call: actor=%+v query=%+v", service.actor, service.query)
	}
	item := envelope.Data.Items[0]
	if envelope.Status != "success" || envelope.Data.Total != 7 || envelope.Data.Page != 2 || envelope.Data.PageSize != 5 ||
		item.JobID != 42 || item.JobTitle != "Campus survey" || item.Creator.Name != "Alex Creator" || item.Marketer.UserID != 23 ||
		item.AcceptedPrice != "2500.00" || item.PaymentMethod != domain.PaymentMethodPromptPay || item.PayoutStatus != "Released" ||
		item.PaidAt == nil || !item.PaidAt.Equal(paidAt) {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}

func TestTransactionHandlerReturnsEmptyListNotNull(t *testing.T) {
	app := transactionHandlerApp(&fakeTransactionService{page: ports.TransactionPage{Page: 1, PageSize: 10}}, true)
	res, err := app.Test(httptest.NewRequest("GET", "/transactions", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var raw struct {
		Data struct {
			Items json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Data.Items) != "[]" {
		t.Fatalf("expected an empty array for the empty state, got %s", raw.Data.Items)
	}
}

func TestTransactionHandlerRejectsBadPagingAndServiceValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		url     string
		service *fakeTransactionService
	}{
		"page zero":      {url: "/transactions?page=0", service: &fakeTransactionService{}},
		"page not int":   {url: "/transactions?page=abc", service: &fakeTransactionService{}},
		"page size high": {url: "/transactions?page_size=1000", service: &fakeTransactionService{}},
		"invalid role":   {url: "/transactions?role=admin", service: &fakeTransactionService{err: domain.ErrInvalidTransactionQuery}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := transactionHandlerApp(tc.service, true).Test(httptest.NewRequest("GET", tc.url, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			if res.StatusCode != fiber.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d", res.StatusCode)
			}
		})
	}
}

func TestTransactionHandlerRequiresAuthentication(t *testing.T) {
	service := &fakeTransactionService{err: errors.New("must not be called")}
	res, err := transactionHandlerApp(service, false).Test(httptest.NewRequest("GET", "/transactions", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnauthorized || service.called {
		t.Fatalf("expected 401 without a service call, got %d called=%t", res.StatusCode, service.called)
	}
}
