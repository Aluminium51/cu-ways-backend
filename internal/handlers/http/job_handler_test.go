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
)

type fakeJobService struct {
	job         *domain.Job
	createInput services.CreateJobInput
	createCalls int
	briefCalls  int
	briefURL    string
	briefs      []domain.Attachment
	surveys     []domain.Survey
	requests    []domain.JobRequest
}

func (f *fakeJobService) ListSurveys(context.Context, services.Actor) ([]domain.Survey, error) {
	return f.surveys, nil
}

func (f *fakeJobService) Create(_ context.Context, actor services.Actor, input services.CreateJobInput) (*domain.Job, error) {
	f.createCalls++
	f.createInput = input
	f.job = &domain.Job{JobID: 12, UserID: actor.UserID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now(), Surveys: []domain.Survey{{SurveyID: input.SurveyIDs[0]}}}
	return f.job, nil
}

func (f *fakeJobService) AddSurvey(context.Context, services.Actor, int32, int32) error { return nil }
func (f *fakeJobService) RemoveSurvey(context.Context, services.Actor, int32, int32) error {
	return nil
}

func (f *fakeJobService) AddBrief(_ context.Context, _ services.Actor, jobID int32, rawURL string) (*domain.Attachment, error) {
	f.briefCalls++
	f.briefURL = rawURL
	return &domain.Attachment{AttachmentID: 3, JobID: jobID, AttachmentType: domain.AttachmentTypeBrief, URL: rawURL, UploadAt: time.Now()}, nil
}

func (f *fakeJobService) ListBriefs(context.Context, services.Actor, int32) ([]domain.Attachment, error) {
	return f.briefs, nil
}

func (f *fakeJobService) CreateRequests(_ context.Context, _ services.Actor, jobID int32, input services.CreateJobRequestsInput) ([]domain.JobRequest, error) {
	for _, marketerID := range input.MarketerIDs {
		f.requests = append(f.requests, domain.JobRequest{JobID: jobID, MarketerID: marketerID, Status: domain.JobRequestStatusPending, CreatedAt: time.Now()})
	}
	return f.requests, nil
}

func jobHandlerApp(service *fakeJobService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
		return c.Next()
	})
	handler := NewJobHandler(service)
	app.Get("/me/surveys", handler.ListSurveys)
	app.Post("/jobs", handler.Create)
	app.Post("/jobs/:id/briefs", handler.AddBrief)
	return app
}

func TestJobHandlerCreatesJobFromSurveyIDsAndReturnsInitialState(t *testing.T) {
	service := &fakeJobService{}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs", strings.NewReader(`{"survey_ids":[4,9]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}
	if service.createCalls != 1 || len(service.createInput.SurveyIDs) != 2 || service.job.UserID != 8 || service.job.JobStatus != domain.JobStatusPending {
		t.Fatalf("unexpected service call: %+v", service)
	}
	var envelope struct {
		Data JobResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.JobID != 12 || envelope.Data.AcceptedOfferID != nil || len(envelope.Data.SurveyIDs) != 1 || envelope.Data.SurveyIDs[0] != 4 {
		t.Fatalf("unexpected job response: %+v", envelope.Data)
	}
}

func TestJobHandlerRejectsEmptyAndUnknownCreateFields(t *testing.T) {
	for _, body := range []string{`{"survey_ids":[]}`, `{"survey_ids":[1],"user_id":8}`} {
		service := &fakeJobService{}
		res, err := jobHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs", strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != fiber.StatusUnprocessableEntity {
			t.Fatalf("body %s: expected 422, got %d", body, res.StatusCode)
		}
		closeResponseBody(t, res.Body)
		if service.createCalls != 0 {
			t.Fatal("invalid body reached service")
		}
	}
}

func TestJobHandlerReturnsOnlyAuthenticatedCreatorsSurveysAsArray(t *testing.T) {
	service := &fakeJobService{surveys: []domain.Survey{{SurveyID: 4, UserID: 8, Title: "Owned"}}}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("GET", "/me/surveys", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data []SurveyResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].SurveyID != 4 {
		t.Fatalf("unexpected survey list: %+v", envelope.Data)
	}
}

func TestJobHandlerRejectsInvalidBriefURLBeforeCallingService(t *testing.T) {
	service := &fakeJobService{}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs/12/briefs", strings.NewReader(`{"url":"javascript:alert(1)"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusUnprocessableEntity || service.briefCalls != 0 {
		t.Fatalf("expected invalid URL rejection before service, got status=%d calls=%d", res.StatusCode, service.briefCalls)
	}
}

func TestMapJobErrorReportsHiredJobAsConflict(t *testing.T) {
	var appErr *response.AppError
	if !errors.As(mapJobError(domain.ErrJobNotPending), &appErr) || appErr.Status != fiber.StatusConflict || appErr.Code != "job_not_pending" {
		t.Fatalf("expected 409 job_not_pending, got %+v", appErr)
	}
}
