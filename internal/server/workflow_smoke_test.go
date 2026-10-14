package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type workflowReadinessChecker struct {
	err   error
	calls int
}

func (c *workflowReadinessChecker) Ping(context.Context) error {
	c.calls++
	return c.err
}

// TestWorkflowSmoke proves the real HTTP composition without connecting to a
// database. Protected requests must stop at authentication before persistence.
func TestWorkflowSmoke(t *testing.T) {
	const dependencyError = "simulated PostgreSQL connectivity failure"
	checks := []struct {
		name          string
		method        string
		path          string
		dependencyErr error
		status        int
		envelope      string
		dataStatus    string
		errorCode     string
		pings         int
	}{
		{name: "liveness", path: "/healthz", status: 200, envelope: "success", dataStatus: "ok"},
		{name: "liveness_with_dependency_down", path: "/healthz", dependencyErr: errors.New(dependencyError), status: 200, envelope: "success", dataStatus: "ok"},
		{name: "readiness", path: "/readyz", status: 200, envelope: "success", dataStatus: "ready", pings: 1},
		{name: "readiness_with_dependency_down", path: "/readyz", dependencyErr: errors.New(dependencyError), status: 503, envelope: "error", errorCode: "database_unavailable", pings: 1},
		{name: "users_require_authentication", path: "/api/v1/users", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "profile_requires_authentication", path: "/api/v1/me/marketer-profile", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "services_require_authentication", path: "/api/v1/me/services", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "statistics_require_authentication", path: "/api/v1/me/statistics", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "search_requires_authentication", path: "/api/v1/marketers", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "survey_requires_authentication", path: "/api/v1/surveys/1", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "creator_surveys_require_authentication", path: "/api/v1/me/surveys", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_creation_requires_authentication", method: "POST", path: "/api/v1/jobs", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_survey_add_requires_authentication", method: "POST", path: "/api/v1/jobs/1/surveys/1", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_survey_remove_requires_authentication", method: "DELETE", path: "/api/v1/jobs/1/surveys/1", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "brief_creation_requires_authentication", method: "POST", path: "/api/v1/jobs/1/briefs", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "brief_history_requires_authentication", path: "/api/v1/jobs/1/briefs", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_invitations_require_authentication", method: "POST", path: "/api/v1/jobs/1/requests", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "marketer_job_requests_require_authentication", path: "/api/v1/me/job-requests", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_request_accept_requires_authentication", method: "POST", path: "/api/v1/me/job-requests/1/accept", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_request_decline_requires_authentication", method: "POST", path: "/api/v1/me/job-requests/1/decline", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "job_request_offers_require_authentication", path: "/api/v1/me/job-requests/1/offers", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "offer_submission_requires_authentication", method: "POST", path: "/api/v1/me/job-requests/1/offers", status: 401, envelope: "error", errorCode: "unauthorized"},
		{name: "unknown_route", path: "/__workflow_smoke_missing", status: 404, envelope: "error", errorCode: "not_found"},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			var logs bytes.Buffer
			checker := &workflowReadinessChecker{err: check.dependencyErr}
			app := New(Dependencies{
				Logger:           zerolog.New(&logs),
				ReadinessChecker: checker,
				ReadinessTimeout: time.Second,
			})
			requestID := "workflow-smoke-" + check.name
			method := check.method
			if method == "" {
				method = "GET"
			}
			request := httptest.NewRequest(method, check.path, nil)
			request.Header.Set("X-Request-ID", requestID)
			res, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			if res.StatusCode != check.status {
				t.Fatalf("%s %s: expected %d, got %d", method, check.path, check.status, res.StatusCode)
			}
			if got := res.Header.Get("X-Request-ID"); got != requestID {
				t.Fatalf("request ID correlation: expected %q, got %q", requestID, got)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Status string `json:"status"`
				Data   struct {
					Status string `json:"status"`
				} `json:"data"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatal("response must be a JSON envelope")
			}
			if envelope.Status != check.envelope || envelope.Data.Status != check.dataStatus || envelope.Error.Code != check.errorCode {
				t.Fatalf("unexpected response contract: status=%q data.status=%q error.code=%q", envelope.Status, envelope.Data.Status, envelope.Error.Code)
			}
			if strings.Contains(string(body), dependencyError) {
				t.Fatal("response exposed internal dependency failure details")
			}
			if checker.calls != check.pings {
				t.Fatalf("expected %d readiness probes, got %d", check.pings, checker.calls)
			}
			if logs.Len() == 0 {
				t.Fatal("expected request diagnostics")
			}
			for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
				var event struct {
					RequestID string `json:"request_id"`
				}
				if err := json.Unmarshal(line, &event); err != nil {
					t.Fatal("request diagnostic must be valid JSON")
				}
				if event.RequestID != requestID {
					t.Fatal("request diagnostic lost request ID correlation")
				}
			}
			t.Logf("smoke method=%s path=%s status=%d request_id=%s dependency_probes=%d", method, check.path, res.StatusCode, requestID, checker.calls)
		})
	}
}
