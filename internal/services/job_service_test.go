package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakeJobRepository struct {
	createdJob        *domain.Job
	createdSurveyIDs  []int32
	createdBrief      *domain.Attachment
	requestIDs        []int32
	requestCreatedAt  time.Time
	createCalls       int
	listJobsCalls     int
	getJobCalls       int
	listRequestsCalls int
	creatorID         int32
	jobID             int32
}

func (f *fakeJobRepository) ListSurveysByCreator(context.Context, int32) ([]domain.Survey, error) {
	return []domain.Survey{{SurveyID: 4}}, nil
}

func (f *fakeJobRepository) ListJobsByCreator(_ context.Context, creatorID int32) ([]domain.CreatorJobSummary, error) {
	f.listJobsCalls++
	f.creatorID = creatorID
	return []domain.CreatorJobSummary{}, nil
}

func (f *fakeJobRepository) GetJobByCreator(_ context.Context, creatorID, jobID int32) (*domain.CreatorJobSummary, error) {
	f.getJobCalls++
	f.creatorID = creatorID
	f.jobID = jobID
	return &domain.CreatorJobSummary{JobID: jobID}, nil
}

func (f *fakeJobRepository) ListJobRequestsByCreator(_ context.Context, creatorID, jobID int32) ([]domain.SentJobRequest, error) {
	f.listRequestsCalls++
	f.creatorID = creatorID
	f.jobID = jobID
	return []domain.SentJobRequest{}, nil
}

func (f *fakeJobRepository) CreateWithSurveys(_ context.Context, job *domain.Job, surveyIDs []int32) (*domain.Job, error) {
	f.createCalls++
	f.createdJob = job
	f.createdSurveyIDs = append([]int32(nil), surveyIDs...)
	job.JobID = 12
	job.Surveys = []domain.Survey{{SurveyID: surveyIDs[0]}}
	return job, nil
}

func (f *fakeJobRepository) AddSurvey(context.Context, int32, int32, int32) error    { return nil }
func (f *fakeJobRepository) RemoveSurvey(context.Context, int32, int32, int32) error { return nil }

func (f *fakeJobRepository) CreateBrief(_ context.Context, _ int32, _ int32, attachment *domain.Attachment) error {
	f.createdBrief = attachment
	attachment.AttachmentID = 5
	return nil
}

func (f *fakeJobRepository) ListBriefs(context.Context, int32, int32) ([]domain.Attachment, error) {
	return []domain.Attachment{}, nil
}

func (f *fakeJobRepository) CreateRequests(_ context.Context, _, jobID int32, marketerIDs []int32, createdAt time.Time) ([]domain.JobRequest, error) {
	f.requestIDs = append([]int32(nil), marketerIDs...)
	f.requestCreatedAt = createdAt
	items := make([]domain.JobRequest, 0, len(marketerIDs))
	for _, marketerID := range marketerIDs {
		items = append(items, domain.JobRequest{JobID: jobID, MarketerID: marketerID, Status: domain.JobRequestStatusPending, CreatedAt: createdAt})
	}
	return items, nil
}

func TestJobServiceCreatesPendingJobWithAuthenticatedCreatorAndSelectedSurveys(t *testing.T) {
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{creator: true})
	job, err := service.Create(context.Background(), Actor{UserID: 8}, CreateJobInput{SurveyIDs: []int32{4, 9}})
	if err != nil {
		t.Fatal(err)
	}
	if job.UserID != 8 || job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil || job.CreatedAt.IsZero() {
		t.Fatalf("unexpected initial job state: %+v", job)
	}
	if len(repo.createdSurveyIDs) != 2 || repo.createdSurveyIDs[0] != 4 || repo.createdSurveyIDs[1] != 9 {
		t.Fatalf("unexpected selected surveys: %v", repo.createdSurveyIDs)
	}
}

func TestJobServiceRejectsMissingOrDuplicateSurveysBeforePersistence(t *testing.T) {
	for _, ids := range [][]int32{nil, {}, {0}, {-1}, {2, 2}} {
		repo := &fakeJobRepository{}
		service := NewJobService(repo, &fakeMembershipRepository{creator: true})
		if _, err := service.Create(context.Background(), Actor{UserID: 8}, CreateJobInput{SurveyIDs: ids}); !errors.Is(err, domain.ErrNoJobSurveys) && !errors.Is(err, domain.ErrInvalidJob) {
			t.Fatalf("expected invalid selection for %v, got %v", ids, err)
		}
		if repo.createCalls != 0 {
			t.Fatalf("invalid selection %v reached persistence", ids)
		}
	}
}

func TestJobServiceRequiresCreatorMembershipAndUsesActorIdentity(t *testing.T) {
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{})
	if _, err := service.ListSurveys(context.Background(), Actor{UserID: 8}); !errors.Is(err, domain.ErrCreatorRequired) {
		t.Fatalf("expected creator membership requirement, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatal("non-Creator reached repository")
	}
}

func TestJobServiceReadMethodsRequireCreatorAndValidateJobIDs(t *testing.T) {
	ctx := context.Background()
	actor := Actor{UserID: 8}
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{creator: true})

	jobs, err := service.ListMyJobs(ctx, actor)
	if err != nil || jobs == nil || repo.listJobsCalls != 1 || repo.creatorID != actor.UserID {
		t.Fatalf("expected authenticated creator's jobs, got jobs=%+v err=%v repo=%+v", jobs, err, repo)
	}
	job, err := service.GetJob(ctx, actor, 12)
	if err != nil || job == nil || job.JobID != 12 || repo.getJobCalls != 1 || repo.creatorID != actor.UserID || repo.jobID != 12 {
		t.Fatalf("expected requested owned job to reach repository, got job=%+v err=%v repo=%+v", job, err, repo)
	}
	requests, err := service.ListJobRequests(ctx, actor, 12)
	if err != nil || requests == nil || repo.listRequestsCalls != 1 || repo.creatorID != actor.UserID || repo.jobID != 12 {
		t.Fatalf("expected request history for authenticated creator's job, got requests=%+v err=%v repo=%+v", requests, err, repo)
	}

	for _, invalidID := range []int32{0, -1} {
		if _, err := service.GetJob(ctx, actor, invalidID); !errors.Is(err, domain.ErrInvalidJob) {
			t.Fatalf("GetJob(%d) expected invalid job error, got %v", invalidID, err)
		}
		if _, err := service.ListJobRequests(ctx, actor, invalidID); !errors.Is(err, domain.ErrInvalidJob) {
			t.Fatalf("ListJobRequests(%d) expected invalid job error, got %v", invalidID, err)
		}
	}
	if repo.getJobCalls != 1 || repo.listRequestsCalls != 1 {
		t.Fatalf("invalid job IDs reached repository: %+v", repo)
	}

	deniedRepo := &fakeJobRepository{}
	deniedService := NewJobService(deniedRepo, &fakeMembershipRepository{})
	if _, err := deniedService.ListMyJobs(ctx, actor); !errors.Is(err, domain.ErrCreatorRequired) {
		t.Fatalf("expected Creator membership denial for list, got %v", err)
	}
	if _, err := deniedService.GetJob(ctx, actor, 12); !errors.Is(err, domain.ErrCreatorRequired) {
		t.Fatalf("expected Creator membership denial for detail, got %v", err)
	}
	if _, err := deniedService.ListJobRequests(ctx, actor, 12); !errors.Is(err, domain.ErrCreatorRequired) {
		t.Fatalf("expected Creator membership denial for requests, got %v", err)
	}
	if deniedRepo.listJobsCalls != 0 || deniedRepo.getJobCalls != 0 || deniedRepo.listRequestsCalls != 0 {
		t.Fatalf("non-Creator reached repository: %+v", deniedRepo)
	}
}

func TestJobServiceValidatesBriefURLAndStoresBriefType(t *testing.T) {
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{creator: true})
	if _, err := service.AddBrief(context.Background(), Actor{UserID: 8}, 12, "javascript:alert(1)"); !errors.Is(err, domain.ErrInvalidBriefURL) {
		t.Fatalf("expected invalid brief URL, got %v", err)
	}
	if repo.createdBrief != nil {
		t.Fatal("invalid URL reached repository")
	}
	brief, err := service.AddBrief(context.Background(), Actor{UserID: 8}, 12, " https://example.com/brief ")
	if err != nil {
		t.Fatal(err)
	}
	if brief.AttachmentType != domain.AttachmentTypeBrief || brief.URL != "https://example.com/brief" || brief.UploadAt.IsZero() {
		t.Fatalf("unexpected brief: %+v", brief)
	}
}

func TestJobServiceCreatesPendingRequestsForValidatedUniqueMarketers(t *testing.T) {
	repo := &fakeJobRepository{}
	service := NewJobService(repo, &fakeMembershipRepository{creator: true})
	requests, err := service.CreateRequests(context.Background(), Actor{UserID: 8}, 12, CreateJobRequestsInput{MarketerIDs: []int32{21, 22}})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].Status != domain.JobRequestStatusPending || requests[0].RespondedAt != nil || repo.requestCreatedAt.IsZero() {
		t.Fatalf("unexpected requests: %+v", requests)
	}
	if _, err := service.CreateRequests(context.Background(), Actor{UserID: 8}, 12, CreateJobRequestsInput{MarketerIDs: []int32{21, 21}}); !errors.Is(err, domain.ErrInvalidJobRequest) {
		t.Fatalf("expected duplicate marketer IDs to be rejected, got %v", err)
	}
}
