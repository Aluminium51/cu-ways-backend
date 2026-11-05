package services

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type SubmitReviewInput struct {
	Rating  int32
	Comment *string
}

type ReviewService struct {
	repo        ports.ReviewRepository
	memberships ports.MembershipRepository
}

func NewReviewService(repo ports.ReviewRepository, memberships ports.MembershipRepository) *ReviewService {
	return &ReviewService{repo: repo, memberships: memberships}
}

func (s *ReviewService) Submit(ctx context.Context, actor Actor, jobID int32, input SubmitReviewInput) (*domain.Review, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrCreatorRequired
	}
	isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !isCreator {
		return nil, domain.ErrCreatorRequired
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	if input.Rating < 1 || input.Rating > 5 {
		return nil, domain.ErrInvalidReview
	}
	return s.repo.CreateForCreatorJob(ctx, actor.UserID, jobID, &domain.Review{
		JobID:     jobID,
		Rating:    input.Rating,
		Comment:   input.Comment,
		CreatedAt: time.Now().UTC(),
	})
}
