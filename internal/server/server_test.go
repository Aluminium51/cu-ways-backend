package server

import (
	"context"
	"io"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type checker struct{}

func closeResponseBody(t *testing.T, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Errorf("close response body: %v", err)
	}
}

func (checker) Ping(context.Context) error {
	return nil
}

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate server test file")
	}

	return New(Dependencies{
		Logger:           zerolog.Nop(),
		ReadinessChecker: checker{},
		ReadinessTimeout: time.Second,
		DocsPath:         filepath.Join(filepath.Dir(currentFile), "..", "..", "docs", "openapi.yaml"),
	})
}

func TestNewAddsRequestID(t *testing.T) {
	app := newTestApp(t)

	res, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}

func TestNewServesOpenAPISpec(t *testing.T) {
	app := newTestApp(t)

	res, err := app.Test(httptest.NewRequest("GET", "/docs/openapi.yaml", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	spec := string(body)
	spec = strings.ReplaceAll(spec, "\r\n", "\n")
	if !strings.Contains(spec, "openapi: 3.0.3") {
		t.Fatal("expected OpenAPI version in served specification")
	}

	for _, expected := range []string{
		"  /api/v1/users:\n    get:\n      operationId: listUsers",
		"  /api/v1/auth/register:\n    post:\n      operationId: register",
		"  /api/v1/users/{id}:\n    parameters:",
		"    get:\n      operationId: getUser",
		"    put:\n      operationId: updateUser",
		"    delete:\n      operationId: deleteUser",
		"  /api/v1/me/marketer-profile:\n    get:",
		"  /api/v1/me/services:\n    get:",
		"  /api/v1/marketers:\n    get:",
		"  /api/v1/surveys:\n    post:",
		"  /api/v1/surveys/{id}:\n    parameters:",
	} {
		if !strings.Contains(spec, expected) {
			t.Errorf("expected served OpenAPI specification to contain %q", expected)
		}
	}
	if strings.Contains(spec, "  /api/v1/users:\n    post:") {
		t.Fatal("expected POST /api/v1/users to be absent from the OpenAPI specification")
	}
}

func TestNewDoesNotRegisterLegacyCreateUserRoute(t *testing.T) {
	app := newTestApp(t)

	res, err := app.Test(httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(`{"name":"Jane Doe","email":"jane@example.com"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)

	if res.StatusCode != fiber.StatusMethodNotAllowed {
		t.Fatalf("expected legacy POST /api/v1/users to return 405, got %d", res.StatusCode)
	}
}

func TestNewServesScalarReference(t *testing.T) {
	app := newTestApp(t)

	res, err := app.Test(httptest.NewRequest("GET", "/docs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, expected := range []string{
		"CU Ways API Reference",
		`data-url="/docs/openapi.yaml"`,
		"https://cdn.jsdelivr.net/npm/@scalar/api-reference",
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("expected Scalar HTML to contain %q", expected)
		}
	}
}
