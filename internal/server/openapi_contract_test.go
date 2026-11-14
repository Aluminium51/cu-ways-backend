package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type documentedOperation struct {
	Responses map[string]struct {
		Content map[string]struct {
			Schema struct {
				Ref string `yaml:"$ref"`
			}
		}
	}
}

func TestOpenAPIContract(t *testing.T) {
	app := newTestApp(t)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("serve OpenAPI: got HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Decode path items separately because they can contain shared parameters.
	var spec struct {
		Paths map[string]map[string]yaml.Node
	}
	if err := yaml.Unmarshal(body, &spec); err != nil {
		t.Fatalf("parse served OpenAPI: %v", err)
	}
	inScope := func(path string) bool {
		return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/") || path == "/healthz" || path == "/readyz"
	}
	type operation struct{ method, path string }
	documented := make(map[operation]documentedOperation)
	for path, item := range spec.Paths {
		if !inScope(path) {
			continue
		}
		for method, node := range item {
			switch method {
			case "get", "head", "post", "put", "patch", "delete", "options", "trace":
				var op documentedOperation
				if err := node.Decode(&op); err != nil {
					t.Fatalf("parse OpenAPI %s %s: %v", method, path, err)
				}
				documented[operation{strings.ToUpper(method), path}] = op
			}
		}
	}

	t.Run("routes", func(t *testing.T) {
		fiberParam := regexp.MustCompile(`:([A-Za-z_][A-Za-z_0-9]*)`)
		runtime := make(map[operation]bool)
		for _, route := range app.GetRoutes(true) {
			if inScope(route.Path) {
				path := fiberParam.ReplaceAllString(route.Path, `{${1}}`)
				runtime[operation{route.Method, path}] = true
			}
		}
		for op := range runtime {
			if _, exists := documented[op]; !exists {
				// Fiber automatically registers HEAD alongside GET.
				if op.method == http.MethodHead && runtime[operation{http.MethodGet, op.path}] {
					continue
				}
				t.Errorf("Fiber route missing from OpenAPI: %s %s", op.method, op.path)
			}
		}
		for op := range documented {
			if !runtime[op] {
				t.Errorf("OpenAPI operation missing from Fiber: %s %s", op.method, op.path)
			}
		}
	})

	t.Run("error responses", func(t *testing.T) {
		for _, tc := range []struct {
			operation
			status string
		}{
			{operation{http.MethodPost, "/api/v1/auth/register"}, "429"},
			{operation{http.MethodPost, "/api/v1/auth/login"}, "429"},
			{operation{http.MethodGet, "/api/v1/jobs/{id}/payments"}, "422"},
			{operation{http.MethodPost, "/api/v1/jobs/{id}/complete"}, "422"},
			{operation{http.MethodPost, "/api/v1/jobs/{id}/complete"}, "409"},
		} {
			response, ok := documented[tc.operation].Responses[tc.status]
			if !ok {
				t.Errorf("OpenAPI %s %s is missing HTTP %s", tc.method, tc.path, tc.status)
				continue
			}
			if got := response.Content["application/json"].Schema.Ref; got != "#/components/schemas/ErrorResponse" {
				t.Errorf("OpenAPI %s %s HTTP %s schema = %q, want ErrorResponse", tc.method, tc.path, tc.status, got)
			}
		}
	})
}
