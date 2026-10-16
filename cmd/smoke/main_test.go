package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSmokeReadOnlyProbeContracts(t *testing.T) {
	checks := []struct {
		name      string
		status    int
		body      string
		requestID bool
		code      int
		result    string
		readiness bool
	}{
		{name: "healthy", status: 200, body: `{"status":"success","data":{"status":"ok"}}`, requestID: true, result: "passed"},
		{name: "not_ready", status: 503, body: `{"status":"error","error":{"code":"database_unavailable"}}`, requestID: true, code: 2, result: "inconclusive", readiness: true},
		{name: "missing_request_id", status: 200, body: `{"status":"success","data":{"status":"ok"}}`, code: 1, result: "failed"},
		{name: "invalid_envelope", status: 200, body: `{"status":"success","data":{"status":"wrong"}}`, requestID: true, code: 1, result: "failed"},
		{name: "body_not_logged", status: 500, body: `fixture-secret-that-must-not-appear`, requestID: true, code: 1, result: "failed"},
		{name: "oversized_body", status: 200, body: strings.Repeat("x", 4097), requestID: true, code: 1, result: "failed"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || (request.URL.Path != "/healthz" && request.URL.Path != "/readyz") || request.Header.Get("Authorization") != "" {
					t.Error("smoke made a request outside the harmless probe contract")
				}
				if check.requestID {
					w.Header().Set("X-Request-ID", request.Header.Get("X-Request-ID"))
				}
				if check.readiness && request.URL.Path == "/healthz" {
					_, _ = w.Write([]byte(`{"status":"success","data":{"status":"ok"}}`))
					return
				}
				w.WriteHeader(check.status)
				_, _ = w.Write([]byte(check.body))
			}))
			defer server.Close()
			var output, diagnostics bytes.Buffer
			args := []string{"-url", server.URL}
			if check.readiness {
				args = append(args, "-readiness")
			}
			if code := run(args, &output, &diagnostics); code != check.code {
				t.Fatalf("exit code=%d, expected=%d", code, check.code)
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			var result smokeResult
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &result); err != nil {
				t.Fatal(err)
			}
			if result.Result != check.result || result.RequestID == "" {
				t.Fatalf("unexpected safe evidence: %+v", result)
			}
			if strings.Contains(output.String()+diagnostics.String(), "fixture-secret") {
				t.Fatal("smoke evidence exposed response contents")
			}
		})
	}
}

func TestSmokeRejectsUnsafeOriginsWithoutRequests(t *testing.T) {
	for _, origin := range []string{
		"https://example.com",
		"http://10.0.0.1",
		"http://fixture-user:fixture-password@localhost:8081",
		"http://localhost:8081/other-path",
		"http://localhost:8081?token=fixture-token",
		"http://localhost:8081#fragment",
		"http://fixture-password%xx@localhost:8081",
	} {
		var output, diagnostics bytes.Buffer
		if code := run([]string{"-url", origin}, &output, &diagnostics); code != 1 {
			t.Fatal("unsafe origin must be rejected")
		}
		if output.Len() != 0 || strings.Contains(diagnostics.String(), "fixture-") {
			t.Fatal("origin validation must not log raw URLs or credentials")
		}
	}
}

func TestSmokeDoesNotFollowRedirects(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		targetCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Request-ID", request.Header.Get("X-Request-ID"))
		http.Redirect(w, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	var output, diagnostics bytes.Buffer
	if code := run([]string{"-url", origin.URL}, &output, &diagnostics); code != 1 || targetCalls != 0 {
		t.Fatal("smoke must reject redirects without probing their target")
	}
}
