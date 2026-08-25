package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type checker struct {
	err error
}

func (c checker) Ping(context.Context) error {
	return c.err
}

func newTestApp(err error) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	handler := NewHealthHandler(services.NewHealthService(checker{err: err}, time.Second))
	app.Get("/healthz", handler.Health)
	app.Get("/readyz", handler.Ready)
	return app
}

func TestHealthReturnsSuccessEnvelope(t *testing.T) {
	res, err := newTestApp(nil).Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var envelope response.SuccessEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "success" {
		t.Fatalf("expected success envelope, got %+v", envelope)
	}
}

func TestReadyReturnsUnavailableEnvelope(t *testing.T) {
	res, err := newTestApp(errors.New("offline")).Test(httptest.NewRequest("GET", "/readyz", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.StatusCode)
	}

	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "database_unavailable" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestErrorHandlerReturnsStandardNotFoundEnvelope(t *testing.T) {
	res, err := newTestApp(nil).Test(httptest.NewRequest("GET", "/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}

	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "error" || envelope.Error.Code != "not_found" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}
