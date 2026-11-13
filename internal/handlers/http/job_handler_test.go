package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	job          *domain.Job
	jobSummary   *domain.CreatorJobSummary
	jobs         []domain.CreatorJobSummary
	createInput  services.CreateJobInput
	createCalls  int
	briefCalls   int
	briefURL     string
	briefs       []domain.Attachment
	surveys      []domain.Survey
	requests     []domain.JobRequest
	sentRequests []domain.SentJobRequest
	cancelCalls  int
	cancelJobID  int32
	cancelInput  services.CancelJobInput
	cancelErr    error
}

func (f *fakeJobService) ListSurveys(context.Context, services.Actor) ([]domain.Survey, error) {
	return f.surveys, nil
}

func (f *fakeJobService) ListMyJobs(context.Context, services.Actor) ([]domain.CreatorJobSummary, error) {
	return f.jobs, nil
}

func (f *fakeJobService) GetJob(context.Context, services.Actor, int32) (*domain.CreatorJobSummary, error) {
	return f.jobSummary, nil
}

func (f *fakeJobService) ListJobRequests(context.Context, services.Actor, int32) ([]domain.SentJobRequest, error) {
	return f.sentRequests, nil
}

func (f *fakeJobService) Create(_ context.Context, actor services.Actor, input services.CreateJobInput) (*domain.Job, error) {
	f.createCalls++
	f.createInput = input
	f.job = &domain.Job{JobID: 12, UserID: actor.UserID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now(), Surveys: []domain.Survey{{SurveyID: input.SurveyIDs[0]}}}
	return f.job, nil
}

func (f *fakeJobService) AddSurvey(context.Context, services.Actor, int32, int32) error { return nil }
func (f *fakeJobService) UpdateDetails(context.Context, services.Actor, int32, services.JobDetailsInput) (*domain.CreatorJobSummary, error) {
	return &domain.CreatorJobSummary{}, nil
}
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

func (f *fakeJobService) Cancel(_ context.Context, _ services.Actor, jobID int32, input services.CancelJobInput) (*ports.JobCancellation, error) {
	f.cancelCalls++
	f.cancelJobID, f.cancelInput = jobID, input
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	cancelledAt := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	return &ports.JobCancellation{
		Job:                 domain.Job{JobID: jobID, JobStatus: domain.JobStatusCancelled, CancelledAt: &cancelledAt, CancelReason: input.Reason},
		ClosedJobRequestIDs: []int32{3, 5},
		RejectedOfferIDs:    []int32{},
	}, nil
}

func jobHandlerApp(service *fakeJobService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: response.ErrorHandler(zerolog.Nop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(middleware.ClaimsLocalKey, &ports.TokenClaims{Subject: "8", Values: map[string]any{}})
		return c.Next()
	})
	handler := NewJobHandler(service)
	app.Get("/me/surveys", handler.ListSurveys)
	app.Get("/me/jobs", handler.ListMyJobs)
	app.Post("/jobs", handler.Create)
	app.Get("/jobs/:id", handler.GetJob)
	app.Get("/jobs/:id/requests", handler.ListJobRequests)
	app.Post("/jobs/:id/briefs", handler.AddBrief)
	app.Post("/jobs/:id/cancel", handler.Cancel)
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

func TestJobHandlerReturnsJobSummariesWithoutPrivateSurveyFields(t *testing.T) {
	createdAt := time.Date(2026, 10, 13, 7, 30, 0, 0, time.UTC)
	service := &fakeJobService{jobs: []domain.CreatorJobSummary{{
		JobID: 12, JobStatus: domain.JobStatusInProgress, CreatedAt: createdAt, PendingOfferCount: 2,
		Surveys: []domain.JobSurveySummary{{SurveyID: 3, Title: "Study"}},
	}}}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("GET", "/me/jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data []CreatorJobSummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].PendingOfferCount != 2 || envelope.Data[0].JobID != 12 || envelope.Data[0].JobStatus != domain.JobStatusInProgress || !envelope.Data[0].CreatedAt.Equal(createdAt) || len(envelope.Data[0].Surveys) != 1 || envelope.Data[0].Surveys[0].SurveyID != 3 || envelope.Data[0].Surveys[0].Title != "Study" {
		t.Fatalf("unexpected job summaries: %+v", envelope.Data)
	}
	for _, forbidden := range []string{"user_id", "accepted_offer_id", "survey_link"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("job summary leaked field %q: %s", forbidden, body)
		}
	}
}

func TestJobHandlerReturnsEmptyMyJobsAsArray(t *testing.T) {
	res, err := jobHandlerApp(&fakeJobService{jobs: []domain.CreatorJobSummary{}}).Test(httptest.NewRequest("GET", "/me/jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data []CreatorJobSummaryResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data == nil || len(envelope.Data) != 0 {
		t.Fatalf("expected an empty JSON array, got %#v", envelope.Data)
	}
}

func TestJobHandlerReturnsOwnedJobSummary(t *testing.T) {
	createdAt := time.Date(2026, 10, 13, 7, 30, 0, 0, time.UTC)
	service := &fakeJobService{jobSummary: &domain.CreatorJobSummary{
		JobID: 12, JobStatus: domain.JobStatusAccepted, CreatedAt: createdAt,
		Surveys: []domain.JobSurveySummary{{SurveyID: 3, Title: "Study"}},
	}}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("GET", "/jobs/12", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data CreatorJobSummaryResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.JobID != 12 || envelope.Data.JobStatus != domain.JobStatusAccepted || !envelope.Data.CreatedAt.Equal(createdAt) || len(envelope.Data.Surveys) != 1 || envelope.Data.Surveys[0].SurveyID != 3 || envelope.Data.Surveys[0].Title != "Study" {
		t.Fatalf("unexpected job summary: %+v", envelope.Data)
	}
}

func TestJobHandlerReturnsNamedInvitationHistoryWithoutContactOrProfileFields(t *testing.T) {
	createdAt := time.Date(2026, 10, 13, 8, 30, 0, 0, time.UTC)
	respondedAt := createdAt.Add(time.Hour)
	service := &fakeJobService{sentRequests: []domain.SentJobRequest{{
		JobRequestID: 4, JobID: 12, MarketerID: 21, Status: domain.JobRequestStatusDeclined,
		CreatedAt: createdAt, RespondedAt: &respondedAt, MarketerName: "Taylor Researcher",
	}}}
	res, err := jobHandlerApp(service).Test(httptest.NewRequest("GET", "/jobs/12/requests", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data []SentJobRequestResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 {
		t.Fatalf("expected one invitation, got %+v", envelope.Data)
	}
	request := envelope.Data[0]
	if request.JobRequestID != 4 || request.JobID != 12 || request.MarketerID != 21 || request.Status != domain.JobRequestStatusDeclined || request.MarketerName != "Taylor Researcher" || request.RespondedAt == nil || !request.RespondedAt.Equal(respondedAt) {
		t.Fatalf("unexpected invitation history: %+v", request)
	}
	for _, forbidden := range []string{"email", "phone", "bio", "expertise", "availability"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("invitation response leaked profile/contact field %q: %s", forbidden, body)
		}
	}
}

func TestJobHandlerReturnsEmptyInvitationHistoryAsArray(t *testing.T) {
	res, err := jobHandlerApp(&fakeJobService{sentRequests: []domain.SentJobRequest{}}).Test(httptest.NewRequest("GET", "/jobs/12/requests", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResponseBody(t, res.Body)
	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	var envelope struct {
		Data []SentJobRequestResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data == nil || len(envelope.Data) != 0 {
		t.Fatalf("expected an empty JSON array, got %#v", envelope.Data)
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

func TestJobHandlerCancelsJobWithOptionalReason(t *testing.T) {
	reason := "Survey no longer needed"
	for _, tt := range []struct {
		name       string
		body       string
		wantReason *string
	}{
		{"empty_body", "", nil},
		{"empty_object", `{}`, nil},
		{"with_reason", `{"cancel_reason":"Survey no longer needed"}`, &reason},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeJobService{}
			res, err := jobHandlerApp(service).Test(httptest.NewRequest("POST", "/jobs/12/cancel", strings.NewReader(tt.body)))
			if err != nil {
				t.Fatal(err)
			}
			defer closeResponseBody(t, res.Body)
			if res.StatusCode != fiber.StatusOK {
				t.Fatalf("expected 200, got %d", res.StatusCode)
			}
			if service.cancelCalls != 1 || service.cancelJobID != 12 {
				t.Fatalf("unexpected service call: %+v", service)
			}
			if (tt.wantReason == nil) != (service.cancelInput.Reason == nil) || (tt.wantReason != nil && *tt.wantReason != *service.cancelInput.Reason) {
				t.Fatalf("expected reason %v, got %v", tt.wantReason, service.cancelInput.Reason)
			}
			var envelope struct {
				Data CancelJobResponse `json:"data"`
			}
			if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			data := envelope.Data
			if data.JobID != 12 || data.JobStatus != domain.JobStatusCancelled || data.CancelledAt == nil || len(data.ClosedJobRequestIDs) != 2 || data.RejectedOfferIDs == nil {
				t.Fatalf("unexpected cancellation response: %+v", data)
			}
		})
	}
}

func TestJobHandlerRejectsInvalidCancelRequestsBeforeCallingService(t *testing.T) {
	for _, tt := range []struct{ path, body string }{
		{"/jobs/0/cancel", ""},
		{"/jobs/abc/cancel", ""},
		{"/jobs/12/cancel", `{"cancel_reason":"x","job_status":"Pending"}`},
		{"/jobs/12/cancel", `{"cancel_reason":`},
	} {
		service := &fakeJobService{}
		res, err := jobHandlerApp(service).Test(httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body)))
		if err != nil {
			t.Fatal(err)
		}
		closeResponseBody(t, res.Body)
		if res.StatusCode != fiber.StatusUnprocessableEntity || service.cancelCalls != 0 {
			t.Fatalf("%s %s: expected 422 before service, got status=%d calls=%d", tt.path, tt.body, res.StatusCode, service.cancelCalls)
		}
	}
}

func TestMapJobErrorReportsCancellationErrors(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrJobAlreadyCancelled, fiber.StatusConflict, "job_already_cancelled"},
		{domain.ErrJobNotCancellable, fiber.StatusConflict, "job_not_cancellable"},
		{domain.ErrJobHasPaidPayment, fiber.StatusConflict, "job_has_paid_payment"},
		{domain.ErrInvalidCancelReason, fiber.StatusUnprocessableEntity, "invalid_cancel_reason"},
		{domain.ErrJobForbidden, fiber.StatusForbidden, "forbidden"},
		{domain.ErrJobNotFound, fiber.StatusNotFound, "resource_not_found"},
	} {
		var appErr *response.AppError
		if !errors.As(mapJobError(tt.err), &appErr) || appErr.Status != tt.status || appErr.Code != tt.code {
			t.Fatalf("%v: expected %d %s, got %+v", tt.err, tt.status, tt.code, appErr)
		}
	}
}
