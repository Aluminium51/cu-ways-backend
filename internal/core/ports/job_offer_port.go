package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// MarketerRating summarizes a Marketer's history: jobs completed through an
// accepted offer and the average of their valid 1-5 reviews. AverageRating
// is nil when the Marketer has no rated jobs yet.
type MarketerRating struct {
	AverageRating *float64
	ReviewCount   int64
	CompletedJobs int64
}

// JobOfferComparison is one offer on a Creator's job together with the
// Marketer profile and rating history needed to compare offers.
type JobOfferComparison struct {
	Offer    domain.Offer
	Marketer domain.Marketer
	Rating   MarketerRating
}

// OfferAcceptance is the outcome of hiring a Marketer through one offer.
type OfferAcceptance struct {
	Job                 domain.Job
	Offer               domain.Offer
	RejectedOfferIDs    []int32
	ClosedJobRequestIDs []int32
}

// JobOfferRepository exposes the persistence operations Creators need to
// review the offers submitted for their jobs and accept one of them.
type JobOfferRepository interface {
	// ListForCreatorJob returns the offers on jobID, oldest first, after
	// checking the job exists (domain.ErrJobNotFound) and belongs to creatorID
	// (domain.ErrJobForbidden). A nil status returns every status.
	ListForCreatorJob(ctx context.Context, creatorID, jobID int32, status *domain.OfferStatus) ([]JobOfferComparison, error)
	// AcceptOffer runs in one transaction that rolls back completely on any
	// failure. It checks job ownership, that the job is still Pending
	// (domain.ErrJobNotPending), that the offer belongs to the job
	// (domain.ErrOfferNotFound), and that the offer is Pending
	// (domain.ErrOfferNotPending). It then marks the job Accepted with
	// accepted_offer_id, accepts the offer, rejects every other Pending offer,
	// and closes the other Marketers' Pending or Accepted job requests.
	AcceptOffer(ctx context.Context, creatorID, jobID, offerID int32) (*OfferAcceptance, error)
}
