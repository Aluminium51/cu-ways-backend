package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type fakeAuthService struct {
	registerResult *services.AuthResult
	registerErr    error
	loginResult    *services.AuthResult
	loginErr       error
	registerCtx    context.Context
	loginCtx       context.Context
}

func (f *fakeAuthService) Register(ctx context.Context, _ services.RegisterInput) (*services.AuthResult, error) {
	f.registerCtx = ctx
	return f.registerResult, f.registerErr
}

func (f *fakeAuthService) Login(ctx context.Context, _ services.LoginInput) (*services.AuthResult, error) {
	f.loginCtx = ctx
	return f.loginResult, f.loginErr
}

func newAuthHandlerTestApp(service authService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	handler := NewAuthHandler(service)
	app.Post("/auth/register", handler.Register)
	app.Post("/auth/login", handler.Login)
	return app
}

func TestAuthHandlerRegisterReturnsTokenAndSanitizedUser(t *testing.T) {
	passwordHash := "must-not-be-exposed"
	service := &fakeAuthService{registerResult: &services.AuthResult{
		User: &domain.User{
			UserID:       1,
			Name:         "Jane Doe",
			Email:        "jane@example.com",
			PasswordHash: &passwordHash,
			CreatedAt:    time.Now().UTC(),
		},
		Token:     "jwt-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	app := newAuthHandlerTestApp(service)

	requestContext := context.WithValue(context.Background(), struct{}{}, "request-value")
	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"name":"Jane Doe","email":"jane@example.com","password":"correct horse battery staple"}`)).WithContext(requestContext)
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	responseBody := string(body)
	for _, expected := range []string{`"access_token":"jwt-token"`, `"token_type":"Bearer"`, `"user_id":1`, `"email":"jane@example.com"`} {
		if !strings.Contains(responseBody, expected) {
			t.Errorf("expected response to contain %q: %s", expected, responseBody)
		}
	}
	if strings.Contains(responseBody, "must-not-be-exposed") || strings.Contains(responseBody, "password_hash") {
		t.Fatalf("password data leaked in response: %s", responseBody)
	}
}

func TestAuthHandlerLoginReturnsUnauthorizedForInvalidCredentials(t *testing.T) {
	service := &fakeAuthService{loginErr: domain.ErrInvalidCredentials}
	app := newAuthHandlerTestApp(service)
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"jane@example.com","password":"wrong password"}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}

	var envelope struct {
		Status string `json:"status"`
		Error  struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Status != "error" || envelope.Error.Code != "invalid_credentials" || envelope.Error.Message != "invalid email or password" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestAuthHandlerRejectsInvalidRequest(t *testing.T) {
	service := &fakeAuthService{}
	app := newAuthHandlerTestApp(service)
	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"name":"Jane"}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", res.StatusCode)
	}
}

func TestAuthHandlerMapsDuplicateEmailToConflict(t *testing.T) {
	service := &fakeAuthService{registerErr: domain.ErrEmailAlreadyExists}
	app := newAuthHandlerTestApp(service)
	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{"name":"Jane","email":"jane@example.com","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusConflict {
		t.Fatalf("expected 409, got %d", res.StatusCode)
	}
}

func TestAuthHandlerPropagatesRequestContext(t *testing.T) {
	service := &fakeAuthService{loginErr: errors.New("stop after context capture")}
	ctx := context.WithValue(context.Background(), struct{}{}, "request-value")
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(ctx)
		return c.Next()
	})
	app.Post("/auth/login", NewAuthHandler(service).Login)
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"jane@example.com","password":"correct horse battery staple"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	_, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if service.loginCtx == nil || service.loginCtx.Value(struct{}{}) != "request-value" {
		t.Fatal("expected request context to reach auth service")
	}
}
