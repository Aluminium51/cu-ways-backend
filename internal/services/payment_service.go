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

func (s *PaymentService) Create(ctx context.Context, actor Actor, jobID int32, input CreatePaymentInput) (*domain.Payment, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrCreatorRequired
	}
	creator, err := s.memberships.IsCreator(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !creator {
		return nil, domain.ErrCreatorRequired
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
		JobID: jobID, Method: input.Method, Amount: amount, PaymentStatus: domain.PaymentStatusPending, CreatedAt: &now,
	})
}
