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

// JobOfferRepository exposes the persistence operations Creators need to
// review the offers submitted for their jobs.
type JobOfferRepository interface {
	// ListForCreatorJob returns the offers on jobID, oldest first, after
	// checking the job exists (domain.ErrJobNotFound) and belongs to creatorID
	// (domain.ErrJobForbidden). A nil status returns every status.
	ListForCreatorJob(ctx context.Context, creatorID, jobID int32, status *domain.OfferStatus) ([]JobOfferComparison, error)
}
