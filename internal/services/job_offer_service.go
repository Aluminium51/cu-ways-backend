package services

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

// JobOfferService lets Creators review the offers Marketers submitted for
// their jobs.
type JobOfferService struct {
	repo        ports.JobOfferRepository
	memberships ports.MembershipRepository
}

func NewJobOfferService(repo ports.JobOfferRepository, memberships ports.MembershipRepository) *JobOfferService {
	return &JobOfferService{repo: repo, memberships: memberships}
}

// ListOffers returns the offers on a job the Creator owns, with each
// Marketer's profile and rating history so the offers can be compared.
func (s *JobOfferService) ListOffers(ctx context.Context, actor Actor, jobID int32, status *domain.OfferStatus) ([]ports.JobOfferComparison, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	if status != nil && !status.IsValid() {
		return nil, domain.ErrInvalidOffer
	}
	return s.repo.ListForCreatorJob(ctx, actor.UserID, jobID, status)
}

func (s *JobOfferService) requireCreator(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrCreatorRequired
	}
	isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isCreator {
		return domain.ErrCreatorRequired
	}
	return nil
}
