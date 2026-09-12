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
	detail  *domain.MarketerDetail
}

func (f *fakeMarketerService) GetProfile(context.Context, services.Actor) (*domain.Marketer, error) {
	return f.profile, nil
}

func (f *fakeMarketerService) GetDetail(context.Context, services.Actor, int32) (*domain.MarketerDetail, error) {
	return f.detail, nil
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

func TestMarketerHandlerReturnsDetailedProfileAndPerformance(t *testing.T) {
	average := 4.5
	service := &fakeMarketerService{detail: &domain.MarketerDetail{
		Marketer: domain.Marketer{
			UserID: 18,
			Bio:    "Survey specialist",
			Services: []domain.Service{{
				ServiceID: 3, ServiceType: "Collection", Price: decimal.NewFromInt(500),
			}},
		},
		TotalCompletedJobs: 2,
		AverageRating:      &average,
	}}
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Get("/marketers/:id", NewMarketerHandler(service).GetDetail)

	res, err := app.Test(httptest.NewRequest("GET", "/marketers/18", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data MarketerDetailResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Profile.UserID != 18 || len(envelope.Data.Services) != 1 || envelope.Data.TotalCompletedJobs != 2 || envelope.Data.AverageRating == nil || *envelope.Data.AverageRating != 4.5 {
		t.Fatalf("unexpected detail response: %+v", envelope.Data)
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

type fakeFeatureSurveyService struct {
	createCalls int
	updateCalls int
}

func (f *fakeFeatureSurveyService) Create(context.Context, services.Actor, services.CreateSurveyInput) (*domain.Survey, error) {
	f.createCalls++
	return &domain.Survey{SurveyID: 1, UserID: 1, Title: "Survey", SurveyLink: "https://example.com", CreatedAt: time.Now()}, nil
}

func (f *fakeFeatureSurveyService) Get(context.Context, services.Actor, int32) (*domain.Survey, error) {
	return nil, errors.New("not configured")
}

func (f *fakeFeatureSurveyService) Update(context.Context, services.Actor, int32, services.UpdateSurveyInput) (*domain.Survey, error) {
	f.updateCalls++
	return nil, errors.New("not configured")
}

func (f *fakeFeatureSurveyService) Delete(context.Context, services.Actor, int32) error { return nil }

func TestSurveyHandlerCreatesSurveyWithAuthenticatedContext(t *testing.T) {
	service := &fakeFeatureSurveyService{}
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/surveys", NewSurveyHandler(service).Create)
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
	if service.createCalls != 1 {
		t.Fatalf("expected create service to be called once, got %d", service.createCalls)
	}
}

func TestSurveyHandlerRejectsUnknownCreateFields(t *testing.T) {
	service := &fakeFeatureSurveyService{}
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/surveys", NewSurveyHandler(service).Create)
	request := httptest.NewRequest("POST", "/surveys", strings.NewReader(`{"title":"Survey","survey_link":"https://example.com","questions":[]}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", res.StatusCode)
	}
	if service.createCalls != 0 {
		t.Fatalf("expected create service not to be called, got %d calls", service.createCalls)
	}
}

func TestSurveyHandlerRejectsUnknownUpdateFields(t *testing.T) {
	service := &fakeFeatureSurveyService{}
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "1", Values: map[string]any{}})
		return c.Next()
	})
	app.Patch("/surveys/:id", NewSurveyHandler(service).Update)
	request := httptest.NewRequest("PATCH", "/surveys/1", strings.NewReader(`{"title":"Updated Survey","answers":[]}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", res.StatusCode)
	}
	if service.updateCalls != 0 {
		t.Fatalf("expected update service not to be called, got %d calls", service.updateCalls)
	}
}
