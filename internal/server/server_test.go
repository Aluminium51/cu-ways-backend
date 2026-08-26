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
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	spec := string(body)
	if !strings.Contains(spec, "openapi: 3.0.3") {
		t.Fatal("expected OpenAPI version in served specification")
	}

	for _, expected := range []string{
		"  /api/v1/users:\n    post:",
		"    get:\n      operationId: listUsers",
		"  /api/v1/users/{id}:\n    parameters:",
		"    get:\n      operationId: getUser",
		"    put:\n      operationId: updateUser",
		"    delete:\n      operationId: deleteUser",
	} {
		if !strings.Contains(spec, expected) {
			t.Errorf("expected served OpenAPI specification to contain %q", expected)
		}
	}
}

func TestNewServesScalarReference(t *testing.T) {
	app := newTestApp(t)

	res, err := app.Test(httptest.NewRequest("GET", "/docs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

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
