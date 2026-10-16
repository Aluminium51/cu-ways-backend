package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// JobRepository exposes the persistence operations needed by Creator job
// workflows. Multi-row invariants are atomic inside each implementation.
type JobRepository interface {
	ListSurveysByCreator(ctx context.Context, creatorID int32) ([]domain.Survey, error)
	CreateWithSurveys(ctx context.Context, job *domain.Job, surveyIDs []int32) (*domain.Job, error)
	AddSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error
	RemoveSurvey(ctx context.Context, creatorID, jobID, surveyID int32) error
	CreateBrief(ctx context.Context, creatorID, jobID int32, attachment *domain.Attachment) error
	ListBriefs(ctx context.Context, creatorID, jobID int32) ([]domain.Attachment, error)
	CreateRequests(ctx context.Context, creatorID, jobID int32, marketerIDs []int32, createdAt time.Time) ([]domain.JobRequest, error)
}
