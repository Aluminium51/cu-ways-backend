package services

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type JobSettlementService struct {
	repo        ports.JobSettlementRepository
	memberships ports.MembershipRepository
}

func NewJobSettlementService(repo ports.JobSettlementRepository, memberships ports.MembershipRepository) *JobSettlementService {
	return &JobSettlementService{repo: repo, memberships: memberships}
}

func (s *JobSettlementService) GetPayment(ctx context.Context, actor Actor, jobID int32) (*domain.JobSettlement, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrJobForbidden
	}
	return s.repo.GetPayment(ctx, actor.UserID, jobID)
}

func (s *JobSettlementService) Complete(ctx context.Context, actor Actor, jobID int32) (*domain.JobSettlement, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.Complete(ctx, actor.UserID, jobID)
}

func (s *JobSettlementService) requireCreator(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrCreatorRequired
	}
	ok, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrCreatorRequired
	}
	return nil
}
