package services

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type CreatePaymentInput struct {
	Method domain.PaymentMethod
	Amount string
}

type PaymentService struct {
	repo        ports.PaymentRepository
	memberships ports.MembershipRepository
}

func NewPaymentService(repo ports.PaymentRepository, memberships ports.MembershipRepository) *PaymentService {
	return &PaymentService{repo: repo, memberships: memberships}
}

func (s *PaymentService) requireCreator(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrCreatorRequired
	}
	creator, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !creator {
		return domain.ErrCreatorRequired
	}
	return nil
}

// Current returns the active (Pending or Paid) payment on a Job the Creator
// owns, or nil when the Job has none yet.
func (s *PaymentService) Current(ctx context.Context, actor Actor, jobID int32) (*domain.Payment, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.FindActiveForCreatorJob(ctx, actor.UserID, jobID)
}

func (s *PaymentService) Create(ctx context.Context, actor Actor, jobID int32, input CreatePaymentInput) (*domain.Payment, error) {
	if err := s.requireCreator(ctx, actor); err != nil {
		return nil, err
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	if !input.Method.IsSupported() {
		return nil, domain.ErrInvalidPayment
	}
	amount, err := parsePrice(input.Amount)
	if err != nil {
		return nil, domain.ErrInvalidPayment
	}
	now := time.Now().UTC()
	return s.repo.CreateForCreatorJob(ctx, actor.UserID, jobID, &domain.Payment{
		JobID: jobID, Method: input.Method, Amount: amount, PaymentStatus: domain.PaymentStatusPaid, CreatedAt: &now, PaidAt: &now,
	})
}
