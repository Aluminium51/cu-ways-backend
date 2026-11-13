package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// JobCancellation is the outcome of a Creator cancelling a job: the cancelled
// job and every job request and offer the cancellation closed.
type JobCancellation struct {
	Job                 domain.Job
	ClosedJobRequestIDs []int32
	RejectedOfferIDs    []int32
}

// JobRepository exposes the persistence operations needed by Creator job
// workflows. Multi-row invariants are atomic inside each implementation.
type JobRepository interface {
	ListSurveysByCreator(ctx context.Context, creatorID int32) ([]domain.Survey, error)
	ListJobsByCreator(ctx context.Context, creatorID int32) ([]domain.CreatorJobSummary, error)
	GetJobByCreator(ctx context.Context, creatorID, jobID int32) (*domain.CreatorJobSummary, error)
	ListJobRequestsByCreator(ctx context.Context, creatorID, jobID int32) ([]domain.SentJobRequest, error)
	CreateWithSurveys(ctx context.Context, job *domain.Job, surveyIDs []int32) (*domain.Job, error)
	UpdateDetails(ctx context.Context, creatorID, jobID int32, details *domain.Job) error
	AddSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error
	RemoveSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error
	CreateBrief(ctx context.Context, creatorID, jobID int32, attachment *domain.Attachment) error
	ListBriefs(ctx context.Context, creatorID, jobID int32) ([]domain.Attachment, error)
	CreateRequests(ctx context.Context, creatorID, jobID int32, marketerIDs []int32, createdAt time.Time) ([]domain.JobRequest, error)
	// Cancel runs in one transaction that rolls back completely on any
	// failure. It checks job ownership, applies domain.Job.Cancel (Pending
	// only, no Paid payment), closes every Pending or Accepted job request,
	// and rejects every Pending offer. Payments are left unchanged.
	Cancel(ctx context.Context, creatorID, jobID int32, reason *string, cancelledAt time.Time) (*JobCancellation, error)
}
