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
	"github.com/shopspring/decimal"
)

type fakeMarketerJobRequestService struct {
	items        []ports.MarketerJobRequest
	listStatus   *domain.JobRequestStatus
	listCalls    int
	declineCalls int
	declineInput services.DeclineJobRequestInput
	err          error
	offerCalls   int
	offerInput   services.SubmitOfferInput
	offers       []domain.Offer
}

func (f *fakeMarketerJobRequestService) SubmitOffer(_ context.Context, actor services.Actor, requestID int32, input services.SubmitOfferInput) (*domain.Offer, error) {
	f.offerCalls++
	f.offerInput = input
	if f.err != nil {
		return nil, f.err
	}
	delivery := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	return &domain.Offer{
		OfferID: 31, JobID: 4, UserID: actor.UserID, JobRequestID: &requestID,
		OfferedPrice: decimal.RequireFromString(input.OfferedPrice), DeliveryDate: &delivery,
		Message: input.Message, OfferStatus: domain.OfferStatusPending, CreatedAt: time.Now().UTC(),
	}, nil
}

func (f *fakeMarketerJobRequestService) ListOffers(context.Context, services.Actor, int32) ([]domain.Offer, error) {
	return f.offers, f.err
}

func (f *fakeMarketerJobRequestService) List(_ context.Context, _ services.Actor, status *domain.JobRequestStatus) ([]ports.MarketerJobRequest, error) {
	f.listCalls++
	f.listStatus = status
	return f.items, f.err
}

func (f *fakeMarketerJobRequestService) Accept(_ context.Context, actor services.Actor, requestID int32) (*domain.JobRequest, error) {
	if f.err != nil {
		return nil, f.err
	}
	now := time.Now().UTC()
	return &domain.JobRequest{JobRequestID: requestID, JobID: 4, MarketerID: actor.UserID, Status: domain.JobRequestStatusAccepted, RespondedAt: &now}, nil
}

func (f *fakeMarketerJobRequestService) Decline(_ context.Context, actor services.Actor, requestID int32, input services.DeclineJobRequestInput) (*domain.JobRequest, error) {
	f.declineCalls++
	f.declineInput = input
	if f.err != nil {
		return nil, f.err
	}
	now := time.Now().UTC()
	return &domain.JobRequest{JobRequestID: requestID, JobID: 4, MarketerID: actor.UserID, Status: domain.JobRequestStatusDeclined, RespondedAt: &now, DeclineReason: input.Reason, DeclineNote: input.Note}, nil
}

func marketerJobRequestApp(service *fakeMarketerJobRequestService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "9", Values: map[string]any{}})
		return c.Next()
	})
	handler := NewMarketerJobRequestHandler(service)
	app.Get("/me/job-requests", handler.List)
	app.Post("/me/job-requests/:id/accept", handler.Accept)
	app.Post("/me/job-requests/:id/decline", handler.Decline)
	app.Get("/me/job-requests/:id/offers", handler.ListOffers)
	app.Post("/me/job-requests/:id/offers", handler.SubmitOffer)
	return app
}

func TestMarketerJobRequestHandlerSubmitsOfferAndFormatsMoneyAndDate(t *testing.T) {
	service := &fakeMarketerJobRequestService{}
	body := `{"offered_price":"9000","delivery_date":"2026-10-25","message":"Campus booth."}`
	res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("POST", "/me/job-requests/21/offers", strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	if service.offerInput.OfferedPrice != "9000" || service.offerInput.DeliveryDate != "2026-10-25" || service.offerInput.Message == nil {
		t.Fatalf("unexpected service input: %+v", service.offerInput)
	}
	var envelope struct {
		Data OfferResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	offer := envelope.Data
	if offer.OfferedPrice != "9000.00" || offer.DeliveryDate == nil || *offer.DeliveryDate != "2026-10-25" {
		t.Fatalf("price and date must use fixed formats, got %+v", offer)
	}
	if offer.OfferStatus != domain.OfferStatusPending || offer.JobRequestID == nil || *offer.JobRequestID != 21 || offer.MarketerID != 9 {
		t.Fatalf("unexpected offer: %+v", offer)
	}
}

func TestMarketerJobRequestHandlerRejectsMalformedOfferBeforeService(t *testing.T) {
	for name, body := range map[string]string{
		"numeric_price":    `{"offered_price":9000,"delivery_date":"2026-10-25"}`,
		"missing_price":    `{"delivery_date":"2026-10-25"}`,
		"missing_date":     `{"offered_price":"9000"}`,
		"unknown_field":    `{"offered_price":"9000","delivery_date":"2026-10-25","offer_status":"Accepted"}`,
		"invalid_json":     `{`,
		"message_too_long": `{"offered_price":"9000","delivery_date":"2026-10-25","message":"` + strings.Repeat("a", 301) + `"}`,
	} {
		service := &fakeMarketerJobRequestService{}
		res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("POST", "/me/job-requests/21/offers", strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		closeResponseBody(t, res.Body)
		if res.StatusCode != fiber.StatusUnprocessableEntity || service.offerCalls != 0 {
			t.Fatalf("%s: expected 422 before service, got status=%d calls=%d", name, res.StatusCode, service.offerCalls)
		}
	}
}

func TestMarketerJobRequestHandlerMapsOfferErrors(t *testing.T) {
	for _, tt := range []struct {
		err     error
		status  int
		code    string
		message string
	}{
		{domain.ErrJobRequestNotAccepted, fiber.StatusConflict, "job_request_not_accepted", ""},
		{domain.ErrOfferAlreadyExists, fiber.StatusConflict, "offer_already_exists", ""},
		{domain.ErrJobNotPending, fiber.StatusConflict, "job_not_pending", ""},
		{errors.Join(domain.ErrInvalidOffer, errors.New("delivery_date cannot be after the job deadline")), fiber.StatusUnprocessableEntity, "invalid_offer", "delivery_date cannot be after the job deadline"},
	} {
		service := &fakeMarketerJobRequestService{err: tt.err}
		res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("POST", "/me/job-requests/21/offers", strings.NewReader(`{"offered_price":"10","delivery_date":"2026-10-25"}`)))
		if err != nil {
			t.Fatal(err)
		}
		var envelope response.ErrorEnvelope
		decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
		closeResponseBody(t, res.Body)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if res.StatusCode != tt.status || envelope.Error.Code != tt.code {
			t.Fatalf("%v: expected %d/%s, got %d/%s", tt.err, tt.status, tt.code, res.StatusCode, envelope.Error.Code)
		}
		if tt.message != "" && envelope.Error.Message != tt.message {
			t.Fatalf("expected message %q, got %q", tt.message, envelope.Error.Message)
		}
	}
}

func TestMarketerJobRequestHandlerListsOffersNewestFirst(t *testing.T) {
	service := &fakeMarketerJobRequestService{offers: []domain.Offer{
		{OfferID: 32, JobID: 4, UserID: 9, OfferedPrice: decimal.RequireFromString("8500"), OfferStatus: domain.OfferStatusPending},
		{OfferID: 31, JobID: 4, UserID: 9, OfferedPrice: decimal.RequireFromString("9000"), OfferStatus: domain.OfferStatusWithdrawn},
	}}
	res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("GET", "/me/job-requests/21/offers", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	var envelope struct {
		Data []OfferResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != fiber.StatusOK || len(envelope.Data) != 2 || envelope.Data[0].OfferID != 32 || envelope.Data[0].DeliveryDate != nil {
		t.Fatalf("unexpected offers: status=%d data=%+v", res.StatusCode, envelope.Data)
	}
}

func TestMarketerJobRequestHandlerListsRequestsWithJobContext(t *testing.T) {
	early := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	firstTarget, secondTarget := int32(150), int32(200)
	service := &fakeMarketerJobRequestService{items: []ports.MarketerJobRequest{{
		Request:     domain.JobRequest{JobRequestID: 21, JobID: 4, MarketerID: 9, Status: domain.JobRequestStatusPending, CreatedAt: time.Now().UTC()},
		JobStatus:   domain.JobStatusPending,
		CreatorID:   2,
		CreatorName: "Faculty of Engineering",
		Surveys: []domain.Survey{
			{SurveyID: 5, Title: "Mobility", SurveyLink: "https://example.invalid/a", DesiredResponses: &firstTarget, Deadline: &late},
			{SurveyID: 6, Title: "Green Habits", SurveyLink: "https://example.invalid/b", DesiredResponses: &secondTarget, Deadline: &early},
		},
		CurrentBrief: &domain.Attachment{AttachmentID: 3, JobID: 4, AttachmentType: domain.AttachmentTypeBrief, URL: "https://example.invalid/brief", UploadAt: time.Now().UTC()},
	}}}
	res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("GET", "/me/job-requests?status=Pending", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if service.listStatus == nil || *service.listStatus != domain.JobRequestStatusPending {
		t.Fatalf("status filter was not passed: %v", service.listStatus)
	}
	var envelope struct {
		Data []MarketerJobRequestResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected one request, got %d", len(envelope.Data))
	}
	item := envelope.Data[0]
	if item.JobRequestID != 21 || item.Status != domain.JobRequestStatusPending || item.Job.Creator.Name != "Faculty of Engineering" {
		t.Fatalf("unexpected request: %+v", item)
	}
	if item.Job.Deadline == nil || !item.Job.Deadline.Equal(early) {
		t.Fatalf("job deadline must be the earliest survey deadline, got %v", item.Job.Deadline)
	}
	if item.Job.TargetResponses == nil || *item.Job.TargetResponses != 350 {
		t.Fatalf("target responses must sum survey targets, got %v", item.Job.TargetResponses)
	}
	if len(item.Job.Surveys) != 2 || item.Job.Brief == nil || item.Job.Brief.AttachmentID != 3 {
		t.Fatalf("unexpected job context: %+v", item.Job)
	}
}

func TestMarketerJobRequestHandlerRejectsUnknownStatusFilter(t *testing.T) {
	service := &fakeMarketerJobRequestService{}
	res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("GET", "/me/job-requests?status=Archived", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity || service.listCalls != 0 {
		t.Fatalf("expected 422 before service, got status=%d calls=%d", res.StatusCode, service.listCalls)
	}
}

func TestMarketerJobRequestHandlerDeclineAcceptsEmptyAndDetailedBodies(t *testing.T) {
	service := &fakeMarketerJobRequestService{}
	app := marketerJobRequestApp(service)
	res, err := app.Test(httptest.NewRequest("POST", "/me/job-requests/21/decline", nil))
	if err != nil {
		t.Fatal(err)
	}
	closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK || service.declineInput.Reason != nil || service.declineInput.Note != nil {
		t.Fatalf("empty decline must succeed without details, got status=%d input=%+v", res.StatusCode, service.declineInput)
	}

	res, err = app.Test(httptest.NewRequest("POST", "/me/job-requests/21/decline", strings.NewReader(`{"reason":"Fully booked","note":"Next month works."}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data JobRequestResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != domain.JobRequestStatusDeclined || envelope.Data.DeclineReason == nil || *envelope.Data.DeclineReason != "Fully booked" || envelope.Data.RespondedAt == nil {
		t.Fatalf("unexpected decline response: %+v", envelope.Data)
	}
}

func TestMarketerJobRequestHandlerRejectsUnknownDeclineFields(t *testing.T) {
	service := &fakeMarketerJobRequestService{}
	res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("POST", "/me/job-requests/21/decline", strings.NewReader(`{"reason":"Fully booked","status":"Accepted"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity || service.declineCalls != 0 {
		t.Fatalf("expected 422 before service, got status=%d calls=%d", res.StatusCode, service.declineCalls)
	}
}

func TestMarketerJobRequestHandlerMapsStateErrors(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrJobRequestNotFound, fiber.StatusNotFound, "job_request_not_found"},
		{domain.ErrJobRequestNotPending, fiber.StatusConflict, "job_request_not_pending"},
		{domain.ErrMarketerRequired, fiber.StatusForbidden, "marketer_profile_required"},
		{domain.ErrInvalidJobRequest, fiber.StatusUnprocessableEntity, "validation_error"},
	} {
		service := &fakeMarketerJobRequestService{err: tt.err}
		res, err := marketerJobRequestApp(service).Test(httptest.NewRequest("POST", "/me/job-requests/21/accept", nil))
		if err != nil {
			t.Fatal(err)
		}
		var envelope response.ErrorEnvelope
		decodeErr := json.NewDecoder(res.Body).Decode(&envelope)
		closeResponseBody(t, res.Body)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if res.StatusCode != tt.status || envelope.Error.Code != tt.code {
			t.Fatalf("%v: expected %d/%s, got %d/%s", tt.err, tt.status, tt.code, res.StatusCode, envelope.Error.Code)
		}
	}
}
