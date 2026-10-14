package services

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

// DeclineJobRequestInput carries the optional details of a decline.
type DeclineJobRequestInput struct {
	Reason *string
	Note   *string
}

// MarketerJobRequestService lets Marketers review and answer the job requests
// Creators sent to them. Accepting a request signals interest only; it does
// not hire the Marketer or change the job status.
type MarketerJobRequestService struct {
	repo        ports.MarketerJobRequestRepository
	memberships ports.MembershipRepository
	now         func() time.Time
}

func NewMarketerJobRequestService(repo ports.MarketerJobRequestRepository, memberships ports.MembershipRepository) *MarketerJobRequestService {
	return &MarketerJobRequestService{repo: repo, memberships: memberships, now: time.Now}
}

func (s *MarketerJobRequestService) List(ctx context.Context, actor Actor, status *domain.JobRequestStatus) ([]ports.MarketerJobRequest, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if status != nil && !status.IsValid() {
		return nil, domain.ErrInvalidJobRequest
	}
	return s.repo.ListForMarketer(ctx, actor.UserID, status)
}

func (s *MarketerJobRequestService) Accept(ctx context.Context, actor Actor, requestID int32) (*domain.JobRequest, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if requestID < 1 {
		return nil, domain.ErrInvalidJobRequest
	}
	return s.repo.RespondToPending(ctx, actor.UserID, requestID, ports.JobRequestResponse{
		Status:      domain.JobRequestStatusAccepted,
		RespondedAt: s.now().UTC(),
	})
}

func (s *MarketerJobRequestService) Decline(ctx context.Context, actor Actor, requestID int32, input DeclineJobRequestInput) (*domain.JobRequest, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if requestID < 1 {
		return nil, domain.ErrInvalidJobRequest
	}
	reason, err := normalizeDeclineReason(input.Reason)
	if err != nil {
		return nil, err
	}
	note, err := normalizeDeclineNote(input.Note)
	if err != nil {
		return nil, err
	}
	return s.repo.RespondToPending(ctx, actor.UserID, requestID, ports.JobRequestResponse{
		Status:        domain.JobRequestStatusDeclined,
		RespondedAt:   s.now().UTC(),
		DeclineReason: reason,
		DeclineNote:   note,
	})
}

func (s *MarketerJobRequestService) requireMarketer(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrMarketerRequired
	}
	isMarketer, err := s.memberships.IsMarketer(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isMarketer {
		return domain.ErrMarketerRequired
	}
	return nil
}

func normalizeDeclineReason(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	reason := strings.TrimSpace(*value)
	if reason == "" {
		return nil, nil
	}
	if !slices.Contains(domain.JobRequestDeclineReasons, reason) {
		return nil, domain.ErrInvalidJobRequest
	}
	return &reason, nil
}

func normalizeDeclineNote(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	note := strings.TrimSpace(*value)
	if note == "" {
		return nil, nil
	}
	if len([]rune(note)) > domain.MaxJobRequestDeclineNoteLength {
		return nil, domain.ErrInvalidJobRequest
	}
	return &note, nil
}
