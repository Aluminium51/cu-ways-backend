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

type fakeStatisticsService struct {
	result domain.MarketerStatistics
	err    error
}

func (f *fakeStatisticsService) GetMyStatistics(context.Context, services.Actor) (domain.MarketerStatistics, error) {
	return f.result, f.err
}

func statisticsHandlerApp(service statisticsService, authenticated bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if authenticated {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "17", Values: map[string]any{}})
			return c.Next()
		})
	}
	app.Get("/statistics", NewStatisticsHandler(service).GetMyStatistics)
	return app
}

func TestStatisticsHandlerReturnsStatisticsEnvelope(t *testing.T) {
	average := 4.5
	app := statisticsHandlerApp(&fakeStatisticsService{result: domain.MarketerStatistics{
		TotalCompletedJobs: 2,
		AverageRating:      &average,
		TotalEarnings:      decimal.RequireFromString("225.50"),
	}}, true)

	res, err := app.Test(httptest.NewRequest("GET", "/statistics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Status string                     `json:"status"`
		Data   MarketerStatisticsResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "success" || envelope.Data.TotalCompletedJobs != 2 || envelope.Data.AverageRating == nil || *envelope.Data.AverageRating != 4.5 || envelope.Data.TotalEarnings != "225.50" {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}

func TestStatisticsHandlerRejectsNonMarketer(t *testing.T) {
	app := statisticsHandlerApp(&fakeStatisticsService{err: domain.ErrMarketerRequired}, true)
	res, err := app.Test(httptest.NewRequest("GET", "/statistics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.StatusCode)
	}
}

func TestStatisticsHandlerRequiresAuthentication(t *testing.T) {
	app := statisticsHandlerApp(&fakeStatisticsService{err: errors.New("must not be called")}, false)
	res, err := app.Test(httptest.NewRequest("GET", "/statistics", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}
