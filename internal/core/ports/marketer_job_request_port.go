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
	// FindForMarketer returns one request addressed to marketerID with its job
	// context, or domain.ErrJobRequestNotFound.
	FindForMarketer(ctx context.Context, marketerID, requestID int32) (*MarketerJobRequest, error)
	// CreateOffer atomically re-checks that the request is addressed to
	// marketerID, is still Accepted, and has no active offer, then stores
	// offer for the request's job. It sets offer.OfferID, JobID, and
	// JobRequestID.
	CreateOffer(ctx context.Context, marketerID, requestID int32, offer *domain.Offer) error
	// ListOffers returns the marketer's offers for one request, newest first.
	ListOffers(ctx context.Context, marketerID, requestID int32) ([]domain.Offer, error)
	// WithdrawOffer atomically changes only marketerID's Pending offer to
	// Withdrawn, keeping the record as history. Missing or foreign offers return
	// domain.ErrOfferNotFound; resolved offers return domain.ErrOfferNotPending.
	// It must serialize with acceptance without locking the job or request
	// after the offer, which would invert acceptance's lock order.
	WithdrawOffer(ctx context.Context, marketerID, offerID int32) (*domain.Offer, error)
}
