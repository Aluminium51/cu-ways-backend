package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
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

type fakeMarketerReviewService struct {
	page       ports.MarketerReviewPage
	err        error
	actor      services.Actor
	marketerID int32
	page_      int
	pageSize   int
	called     bool
}

func (f *fakeMarketerReviewService) List(_ context.Context, actor services.Actor, marketerID int32, page, pageSize int) (ports.MarketerReviewPage, error) {
	f.called = true
	f.actor = actor
	f.marketerID = marketerID
	f.page_ = page
	f.pageSize = pageSize
	return f.page, f.err
}

func marketerReviewHandlerApp(service marketerReviewService, authenticated bool) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	if authenticated {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "17", Values: map[string]any{}})
			return c.Next()
		})
	}
	app.Get("/marketers/:id/reviews", NewMarketerReviewHandler(service).List)
	return app
}

func TestMarketerReviewHandlerReturnsVerifiedReviewsEnvelope(t *testing.T) {
	createdAt := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	comment := "Clear communication"
	average := 4.5
	service := &fakeMarketerReviewService{page: ports.MarketerReviewPage{
		Page: 2, PageSize: 5, Total: 7, AverageRating: &average,
		Items: []ports.MarketerReview{
			{Review: domain.Review{ReviewID: 9, JobID: 12, Rating: 5, Comment: &comment, CreatedAt: createdAt}, ReviewerName: "Narin Chai"},
			{Review: domain.Review{ReviewID: 8, JobID: 11, Rating: 4, CreatedAt: createdAt}, ReviewerName: "Mali Arun"},
		},
	}}
	app := marketerReviewHandlerApp(service, true)

	res, err := app.Test(httptest.NewRequest("GET", "/marketers/18/reviews?page=2&page_size=5", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Status string                  `json:"status"`
		Data   MarketerReviewsResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if service.actor.UserID != 17 || service.marketerID != 18 || service.page_ != 2 || service.pageSize != 5 {
		t.Fatalf("unexpected service call: actor=%+v marketer=%d page=%d size=%d", service.actor, service.marketerID, service.page_, service.pageSize)
	}
	data := envelope.Data
	if envelope.Status != "success" || data.Total != 7 || data.ReviewCount != 7 || data.Page != 2 || data.PageSize != 5 ||
		data.AverageRating == nil || *data.AverageRating != 4.5 || len(data.Items) != 2 {
		t.Fatalf("unexpected response: %+v", envelope)
	}
	first := data.Items[0]
	if first.ReviewID != 9 || first.JobID != 12 || first.Rating != 5 || first.Comment == nil || *first.Comment != comment || !first.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected first review: %+v", first)
	}
	if first.ReviewerName != "Narin Chai" || data.Items[1].ReviewerName != "Mali Arun" {
		t.Fatalf("expected reviewer_name for each review, got %q and %q", first.ReviewerName, data.Items[1].ReviewerName)
	}
	if data.Items[1].Comment != nil {
		t.Fatalf("expected null comment to stay null, got %v", *data.Items[1].Comment)
	}
}

func TestMarketerReviewHandlerReturnsEmptyStateWithNullAverage(t *testing.T) {
	service := &fakeMarketerReviewService{page: ports.MarketerReviewPage{Page: 1, PageSize: 10, Total: 0, AverageRating: nil}}
	app := marketerReviewHandlerApp(service, true)

	res, err := app.Test(httptest.NewRequest("GET", "/marketers/18/reviews", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var raw struct {
		Data struct {
			Items         json.RawMessage `json:"items"`
			AverageRating json.RawMessage `json:"average_rating"`
			ReviewCount   int64           `json:"review_count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	if string(raw.Data.Items) != "[]" {
		t.Fatalf("expected an empty array for the empty state, got %s", raw.Data.Items)
	}
	if string(raw.Data.AverageRating) != "null" {
		t.Fatalf("expected null average rather than 0.00, got %s", raw.Data.AverageRating)
	}
	if raw.Data.ReviewCount != 0 {
		t.Fatalf("expected review_count 0, got %d", raw.Data.ReviewCount)
	}
}

func TestMarketerReviewHandlerMapsServiceErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		url    string
		err    error
		status int
		called bool
	}{
		"non_creator_forbidden": {url: "/marketers/18/reviews", err: domain.ErrCreatorRequired, status: fiber.StatusForbidden, called: true},
		"marketer_not_found":    {url: "/marketers/18/reviews", err: domain.ErrMarketerProfileNotFound, status: fiber.StatusNotFound, called: true},
		"bad_id_param":          {url: "/marketers/abc/reviews", err: nil, status: fiber.StatusNotFound, called: false},
		"page_zero":             {url: "/marketers/18/reviews?page=0", err: nil, status: fiber.StatusUnprocessableEntity, called: false},
		"page_size_too_high":    {url: "/marketers/18/reviews?page_size=1000", err: nil, status: fiber.StatusUnprocessableEntity, called: false},
	} {
		t.Run(name, func(t *testing.T) {
			service := &fakeMarketerReviewService{err: tc.err}
			res, err := marketerReviewHandlerApp(service, true).Test(httptest.NewRequest("GET", tc.url, nil))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			if res.StatusCode != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, res.StatusCode)
			}
			if service.called != tc.called {
				t.Fatalf("expected service called=%t, got %t", tc.called, service.called)
			}
		})
	}
}

func TestMarketerReviewHandlerRequiresAuthentication(t *testing.T) {
	service := &fakeMarketerReviewService{err: errors.New("must not be called")}
	res, err := marketerReviewHandlerApp(service, false).Test(httptest.NewRequest("GET", "/marketers/18/reviews", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnauthorized || service.called {
		t.Fatalf("expected 401 without a service call, got %d called=%t", res.StatusCode, service.called)
	}
}
