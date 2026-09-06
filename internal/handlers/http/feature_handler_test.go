package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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

type fakeMarketerService struct {
	profile *domain.Marketer
}

func (f *fakeMarketerService) GetProfile(context.Context, services.Actor) (*domain.Marketer, error) {
	return f.profile, nil
}

func (f *fakeMarketerService) SaveProfile(_ context.Context, _ services.Actor, input services.MarketerProfileInput) (*domain.Marketer, error) {
	return &domain.Marketer{UserID: 1, Bio: input.Bio, ExperienceYears: input.ExperienceYears, AvailabilityStatus: domain.AvailabilityStatus(input.AvailabilityStatus), AvailabilityText: input.AvailabilityText}, nil
}

func (f *fakeMarketerService) Search(context.Context, services.Actor, ports.MarketerSearchQuery) (ports.MarketerPage, error) {
	return ports.MarketerPage{}, nil
}

func featureHandlerApp(handler fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Patch("/profile", handler)
	return app
}

func TestMarketerHandlerValidatesProfileBeforeCallingService(t *testing.T) {
	app := featureHandlerApp(NewMarketerHandler(&fakeMarketerService{}).SaveProfile)
	request := httptest.NewRequest("PATCH", "/profile", strings.NewReader(`{"bio":"","experience_years":2,"availability_status":"available","availability_text":"weekdays","expertise":[],"campuses":[]}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", res.StatusCode)
	}
}

type fakeFeatureServiceService struct {
	created bool
}

func (f *fakeFeatureServiceService) List(context.Context, services.Actor) ([]domain.Service, error) {
	return []domain.Service{{ServiceID: 1, ServiceType: "Collection", Price: decimal.NewFromInt(100)}}, nil
}

func (f *fakeFeatureServiceService) Create(context.Context, services.Actor, services.CreateServiceInput) (*domain.Service, error) {
	f.created = true
	return &domain.Service{ServiceID: 1, ServiceType: "Collection", Price: decimal.NewFromInt(100)}, nil
}

func (f *fakeFeatureServiceService) Update(context.Context, services.Actor, int32, services.UpdateServiceInput) (*domain.Service, error) {
	return &domain.Service{ServiceID: 1, ServiceType: "Collection", Price: decimal.NewFromInt(100)}, nil
}

func (f *fakeFeatureServiceService) Delete(context.Context, services.Actor, int32) error { return nil }

func TestServiceHandlerReturnsServiceEnvelope(t *testing.T) {
	service := &fakeFeatureServiceService{}
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/services", NewServiceHandler(service).Create)
	request := httptest.NewRequest("POST", "/services", strings.NewReader(`{"service_type":"Collection","price":"100.00"}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusCreated || !service.created {
		t.Fatalf("expected created service response, status=%d created=%v", res.StatusCode, service.created)
	}
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Price string `json:"price"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "success" || envelope.Data.Price != "100.00" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

type fakeFeatureSurveyService struct{}

func (fakeFeatureSurveyService) Create(context.Context, services.Actor, services.CreateSurveyInput) (*domain.Survey, error) {
	return &domain.Survey{SurveyID: 1, UserID: 1, Title: "Survey", SurveyLink: "https://example.com", CreatedAt: time.Now()}, nil
}

func (fakeFeatureSurveyService) Get(context.Context, services.Actor, int32) (*domain.Survey, error) {
	return nil, errors.New("not configured")
}

func (fakeFeatureSurveyService) Update(context.Context, services.Actor, int32, services.UpdateSurveyInput) (*domain.Survey, error) {
	return nil, errors.New("not configured")
}

func (fakeFeatureSurveyService) Delete(context.Context, services.Actor, int32) error { return nil }

func TestSurveyHandlerCreatesSurveyWithAuthenticatedContext(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/surveys", NewSurveyHandler(fakeFeatureSurveyService{}).Create)
	request := httptest.NewRequest("POST", "/surveys", strings.NewReader(`{"title":"Survey","survey_link":"https://example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
}
