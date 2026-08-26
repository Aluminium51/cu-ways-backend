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
)

type fakeUserService struct {
	createFn func(context.Context, services.CreateUserInput) (*domain.User, error)
	getFn    func(context.Context, services.Actor, int32) (*domain.User, error)
	listFn   func(context.Context, services.Actor, ports.UserListQuery) (ports.UserPage, error)
	updateFn func(context.Context, services.Actor, int32, services.UpdateUserInput) (*domain.User, error)
	deleteFn func(context.Context, services.Actor, int32) (time.Time, error)
}

func (f *fakeUserService) Create(ctx context.Context, input services.CreateUserInput) (*domain.User, error) {
	if f.createFn == nil {
		return nil, errors.New("create function not configured")
	}
	return f.createFn(ctx, input)
}

func (f *fakeUserService) Get(ctx context.Context, actor services.Actor, userID int32) (*domain.User, error) {
	if f.getFn == nil {
		return nil, errors.New("get function not configured")
	}
	return f.getFn(ctx, actor, userID)
}

func (f *fakeUserService) List(ctx context.Context, actor services.Actor, query ports.UserListQuery) (ports.UserPage, error) {
	if f.listFn == nil {
		return ports.UserPage{}, errors.New("list function not configured")
	}
	return f.listFn(ctx, actor, query)
}

func (f *fakeUserService) Update(ctx context.Context, actor services.Actor, userID int32, input services.UpdateUserInput) (*domain.User, error) {
	if f.updateFn == nil {
		return nil, errors.New("update function not configured")
	}
	return f.updateFn(ctx, actor, userID, input)
}

func (f *fakeUserService) Delete(ctx context.Context, actor services.Actor, userID int32) (time.Time, error) {
	if f.deleteFn == nil {
		return time.Time{}, errors.New("delete function not configured")
	}
	return f.deleteFn(ctx, actor, userID)
}

func newUserHandlerTestApp(service userService, claims *ports.TokenClaims) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if claims != nil {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, claims)
			return c.Next()
		})
	}

	handler := NewUserHandler(service)
	app.Post("/users", handler.Create)
	app.Get("/users", handler.List)
	app.Get("/users/:id", handler.Get)
	app.Put("/users/:id", handler.Update)
	app.Delete("/users/:id", handler.Delete)
	return app
}
func TestUserHandlerCreateReturnsCreatedEnvelope(t *testing.T) {
	service := &fakeUserService{
		createFn: func(_ context.Context, input services.CreateUserInput) (*domain.User, error) {
			if input.Name != "Jane Doe" || input.Email != "jane@example.com" {
				t.Fatalf("unexpected create input: %+v", input)
			}
			return &domain.User{UserID: 1, Name: input.Name, Email: input.Email}, nil
		},
	}
	app := newUserHandlerTestApp(service, nil)

	request := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Jane Doe","email":"jane@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	var envelope struct {
		Status string       `json:"status"`
		Data   UserResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "success" || envelope.Data.UserID != 1 {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}

func TestUserHandlerCreateRejectsInvalidRequest(t *testing.T) {
	service := &fakeUserService{}
	app := newUserHandlerTestApp(service, nil)

	request := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"Jane Doe"}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", res.StatusCode)
	}
	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "validation_error" {
		t.Fatalf("unexpected error response: %+v", envelope)
	}
}

func TestUserHandlerRequiresClaimsForProtectedRoutes(t *testing.T) {
	service := &fakeUserService{}
	app := newUserHandlerTestApp(service, nil)

	res, err := app.Test(httptest.NewRequest("GET", "/users/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func TestUserHandlerUpdateCanClearNullableField(t *testing.T) {
	var got services.UpdateUserInput
	service := &fakeUserService{
		updateFn: func(_ context.Context, actor services.Actor, userID int32, input services.UpdateUserInput) (*domain.User, error) {
			if actor.UserID != 1 || userID != 1 {
				t.Fatalf("unexpected actor or user ID: %+v, %d", actor, userID)
			}
			got = input
			return &domain.User{UserID: userID, Name: "Jane", Email: "jane@example.com"}, nil
		},
	}
	claims := &ports.TokenClaims{Subject: "1", Values: map[string]any{}}
	app := newUserHandlerTestApp(service, claims)

	request := httptest.NewRequest("PUT", "/users/1", strings.NewReader(`{"phone":null}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if !got.PhoneSet || got.Phone != nil {
		t.Fatalf("expected phone clear patch, got %+v", got)
	}
}

func TestUserHandlerListParsesPagination(t *testing.T) {
	var gotQuery ports.UserListQuery
	service := &fakeUserService{
		listFn: func(_ context.Context, actor services.Actor, query ports.UserListQuery) (ports.UserPage, error) {
			if !actor.IsAdmin {
				t.Fatal("expected admin actor")
			}
			gotQuery = query
			return ports.UserPage{Page: query.Page, PageSize: query.PageSize, Total: 0, Items: []domain.User{}}, nil
		},
	}
	claims := &ports.TokenClaims{Subject: "9", Values: map[string]any{"role": "admin"}}
	app := newUserHandlerTestApp(service, claims)

	res, err := app.Test(httptest.NewRequest("GET", "/users?page=2&page_size=10", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if gotQuery.Page != 2 || gotQuery.PageSize != 10 {
		t.Fatalf("unexpected pagination query: %+v", gotQuery)
	}
}

func TestUserHandlerDeleteReturnsEnvelope(t *testing.T) {
	service := &fakeUserService{
		deleteFn: func(_ context.Context, actor services.Actor, userID int32) (time.Time, error) {
			if actor.UserID != userID {
				t.Fatal("expected owner actor")
			}
			return time.Now().UTC(), nil
		},
	}
	claims := &ports.TokenClaims{Subject: "4", Values: map[string]any{}}
	app := newUserHandlerTestApp(service, claims)

	res, err := app.Test(httptest.NewRequest("DELETE", "/users/4", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Status string             `json:"status"`
		Data   DeleteUserResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "success" || !envelope.Data.Deleted || envelope.Data.UserID != 4 {
		t.Fatalf("unexpected delete response: %+v", envelope)
	}
}
