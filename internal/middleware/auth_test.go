package middleware

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/pkg/response"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type fakeVerifier struct {
	claims *ports.TokenClaims
	err    error
}

func (f fakeVerifier) Verify(string) (*ports.TokenClaims, error) {
	return f.claims, f.err
}

func TestRequireJWTRejectsMissingHeader(t *testing.T) {
	app := authTestApp(fakeVerifier{})
	res, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	var envelope response.ErrorEnvelope
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "unauthorized" {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}

func TestRequireJWTStoresClaims(t *testing.T) {
	app := authTestApp(fakeVerifier{claims: &ports.TokenClaims{Subject: "user-1"}})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer signed-token")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}

func TestRequireJWTRejectsInvalidToken(t *testing.T) {
	app := authTestApp(fakeVerifier{err: errors.New("bad token")})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer bad-token")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
}

func authTestApp(verifier ports.TokenVerifier) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Get("/protected", RequireJWT(verifier), func(c *fiber.Ctx) error {
		claims, ok := Claims(c)
		if !ok || claims.Subject != "user-1" {
			return errors.New("claims were not stored")
		}
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}
