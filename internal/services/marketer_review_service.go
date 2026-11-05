package services

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	DefaultMarketerReviewPage     = 1
	DefaultMarketerReviewPageSize = 10
	MaxMarketerReviewPageSize     = 50
)

// MarketerReviewService serves the verified reviews shown on a marketer's
// public profile. It reuses the same Creator-or-admin visibility as the
// marketer detail view and confirms the marketer exists before listing.
type MarketerReviewService struct {
	reviews     ports.ReviewRepository
	profiles    ports.MarketerProfileRepository
	memberships ports.MembershipRepository
}

func NewMarketerReviewService(reviews ports.ReviewRepository, profiles ports.MarketerProfileRepository, memberships ports.MembershipRepository) *MarketerReviewService {
	return &MarketerReviewService{reviews: reviews, profiles: profiles, memberships: memberships}
}

// List returns one page of the marketer's verified reviews with the average
// rating over every qualifying review. The marketer is identified by the path
// id; the caller must be a Creator (or admin) just like the profile detail.
func (s *MarketerReviewService) List(ctx context.Context, actor Actor, marketerID int32, page, pageSize int) (ports.MarketerReviewPage, error) {
	if actor.UserID < 1 || marketerID < 1 {
		return ports.MarketerReviewPage{}, domain.ErrMarketerProfileNotFound
	}
	if !actor.IsAdmin {
		isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
		if err != nil {
			return ports.MarketerReviewPage{}, err
		}
		if !isCreator {
			return ports.MarketerReviewPage{}, domain.ErrCreatorRequired
		}
	}
	// A missing marketer is a 404, distinct from a real marketer with no reviews
	// (which returns an empty page and a nil average).
	if _, err := s.profiles.FindProfile(ctx, marketerID); err != nil {
		return ports.MarketerReviewPage{}, err
	}

	if page < 1 {
		page = DefaultMarketerReviewPage
	}
	if pageSize < 1 {
		pageSize = DefaultMarketerReviewPageSize
	}
	if pageSize > MaxMarketerReviewPageSize {
		pageSize = MaxMarketerReviewPageSize
	}

	return s.reviews.ListForMarketer(ctx, ports.MarketerReviewQuery{
		MarketerID: marketerID,
		Page:       page,
		PageSize:   pageSize,
	})
}
