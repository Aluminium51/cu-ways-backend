package services

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

// MarketerJobService lets the Marketer hired through a job's accepted offer
// move the job through execution.
type MarketerJobService struct {
	repo        ports.MarketerJobRepository
	memberships ports.MembershipRepository
}

func NewMarketerJobService(repo ports.MarketerJobRepository, memberships ports.MembershipRepository) *MarketerJobService {
	return &MarketerJobService{repo: repo, memberships: memberships}
}

// StartJob moves an Accepted job to In Progress when the actor owns its
// accepted offer. Persistence rechecks ownership and status under a job lock
// so two concurrent starts cannot both succeed.
func (s *MarketerJobService) StartJob(ctx context.Context, actor Actor, jobID int32) (*ports.StartedJob, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrMarketerRequired
	}
	isMarketer, err := s.memberships.IsMarketer(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !isMarketer {
		return nil, domain.ErrMarketerRequired
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.StartJob(ctx, actor.UserID, jobID)
}
