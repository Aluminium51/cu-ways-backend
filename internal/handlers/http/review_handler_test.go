package httpapi

import (
	"context"
	"encoding/json"
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

type fakeReviewService struct {
	actor services.Actor
	jobID int32
	input services.SubmitReviewInput
	calls int
	err   error
}

func (f *fakeReviewService) Submit(_ context.Context, actor services.Actor, jobID int32, input services.SubmitReviewInput) (*domain.Review, error) {
	f.calls++
	f.actor, f.jobID, f.input = actor, jobID, input
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Review{ReviewID: 31, JobID: jobID, Rating: input.Rating, Comment: input.Comment, CreatedAt: time.Date(2026, 10, 29, 4, 5, 6, 0, time.UTC)}, nil
}

func reviewHandlerApp(service *fakeReviewService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
		return c.Next()
	})
	app.Post("/jobs/:id/reviews", NewReviewHandler(service).Submit)
	return app
}

func TestReviewHandlerReturnsCreatedReviewAndOptionalComment(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		comment *string
	}{
		{name: "with_comment", body: `{"rating":5,"comment":"Clear communication"}`, comment: ptrReviewComment("Clear communication")},
		{name: "without_comment", body: `{"rating":5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeReviewService{}
			res, err := reviewHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs/12/reviews", strings.NewReader(tc.body)))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			if res.StatusCode != 201 || service.calls != 1 || service.actor.UserID != 8 || service.jobID != 12 || service.input.Rating != 5 {
				t.Fatalf("unexpected status or service call: status=%d service=%+v", res.StatusCode, service)
			}
			var envelope struct {
				Status string         `json:"status"`
				Data   ReviewResponse `json:"data"`
			}
			if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Status != "success" || envelope.Data.ReviewID != 31 || envelope.Data.JobID != 12 || envelope.Data.Rating != 5 || !envelope.Data.CreatedAt.Equal(time.Date(2026, 10, 29, 4, 5, 6, 0, time.UTC)) {
				t.Fatalf("unexpected response: %+v", envelope)
			}
			if (envelope.Data.Comment == nil) != (tc.comment == nil) || (tc.comment != nil && *envelope.Data.Comment != *tc.comment) {
				t.Fatalf("unexpected comment: %v", envelope.Data.Comment)
			}
		})
	}
}

func ptrReviewComment(value string) *string { return &value }

func TestReviewHandlerRejectsInvalidBodyBeforeService(t *testing.T) {
	for _, body := range []string{`{}`, `{"rating":0}`, `{"rating":6}`, `{"rating":3,"creator_id":8}`, `{"rating":"5"}`, `{"rating":5} {}`} {
		service := &fakeReviewService{}
		res, err := reviewHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs/12/reviews", strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		defer closeResponseBody(t, res.Body)
		if res.StatusCode != 422 || service.calls != 0 {
			t.Fatalf("body %q: expected 422 without service call, got %d and %d calls", body, res.StatusCode, service.calls)
		}
	}
}

func TestReviewHandlerMapsJobAndReviewErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		err    error
		status int
		code   string
	}{
		{name: "invalid_id", path: "/jobs/0/reviews", status: 422, code: "validation_error"},
		{name: "creator_required", path: "/jobs/12/reviews", err: domain.ErrCreatorRequired, status: 403, code: "creator_required"},
		{name: "foreign", path: "/jobs/12/reviews", err: domain.ErrJobForbidden, status: 403, code: "forbidden"},
		{name: "missing", path: "/jobs/12/reviews", err: domain.ErrJobNotFound, status: 404, code: "job_not_found"},
		{name: "incomplete", path: "/jobs/12/reviews", err: domain.ErrJobNotCompleted, status: 409, code: "job_not_completed"},
		{name: "duplicate", path: "/jobs/12/reviews", err: domain.ErrReviewAlreadyExists, status: 409, code: "review_already_exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeReviewService{err: tc.err}
			res, err := reviewHandlerApp(service).Test(httptest.NewRequest("POST", tc.path, strings.NewReader(`{"rating":4}`)))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != tc.status || envelope.Error.Code != tc.code {
				t.Fatalf("expected %d/%s, got %d/%s", tc.status, tc.code, res.StatusCode, envelope.Error.Code)
			}
		})
	}
}
