package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// MarketerJobRequest is the read model a Marketer sees for one invitation:
// the request itself plus the job context needed to decide on it.
type MarketerJobRequest struct {
	Request      domain.JobRequest
	JobStatus    domain.JobStatus
	CreatorID    int32
	CreatorName  string
	Surveys      []domain.Survey
	CurrentBrief *domain.Attachment
}

// JobRequestResponse describes the Marketer's answer to a Pending request.
type JobRequestResponse struct {
	Status        domain.JobRequestStatus
	RespondedAt   time.Time
	DeclineReason *string
	DeclineNote   *string
}

// MarketerJobRequestRepository exposes the persistence operations needed by
// Marketers to review and answer the job requests addressed to them.
type MarketerJobRequestRepository interface {
	// ListForMarketer returns requests addressed to marketerID, newest first.
	// A nil status returns every status.
	ListForMarketer(ctx context.Context, marketerID int32, status *domain.JobRequestStatus) ([]MarketerJobRequest, error)
	// RespondToPending atomically applies the response only when the request
	// is addressed to marketerID and still Pending. It returns
	// domain.ErrJobRequestNotFound or domain.ErrJobRequestNotPending otherwise.
	RespondToPending(ctx context.Context, marketerID, requestID int32, response JobRequestResponse) (*domain.JobRequest, error)
}
