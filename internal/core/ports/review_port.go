package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// MarketerReviewQuery selects one page of a marketer's verified reviews.
// MarketerID is the profile being viewed, never the authenticated caller.
type MarketerReviewQuery struct {
	MarketerID int32
	Page       int
	PageSize   int
}

// MarketerReviewPage is one page of a marketer's verified reviews plus the
// aggregate rating across every qualifying review. AverageRating is nil when no
// qualifying reviews exist, so callers can show an empty state instead of 0.00.
type MarketerReviewPage struct {
	Items         []domain.Review
	AverageRating *float64
	Total         int64
	Page          int
	PageSize      int
}

type ReviewRepository interface {
	CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, review *domain.Review) (*domain.Review, error)
	// ListForMarketer returns reviews tied to Completed Jobs in which the
	// marketer held the accepted Offer, newest first, with the average rating
	// over all qualifying reviews.
	ListForMarketer(ctx context.Context, query MarketerReviewQuery) (MarketerReviewPage, error)
}
