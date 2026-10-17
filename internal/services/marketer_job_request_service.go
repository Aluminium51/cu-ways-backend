package services

import (
	"context"
	"errors"
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

// SubmitOfferInput carries a Marketer's offer for an accepted job request.
// OfferedPrice is a decimal string and DeliveryDate a YYYY-MM-DD date.
type SubmitOfferInput struct {
	OfferedPrice string
	DeliveryDate string
	Message      *string
}

// offerDateLocation is the calendar used for offer delivery dates. CU Ways
// serves Chulalongkorn University, so "today" and deadlines use Bangkok time.
var offerDateLocation = time.FixedZone("Asia/Bangkok", 7*60*60)

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

// WithdrawOffer releases a Pending offer without changing its invitation or
// deleting its history. Persistence applies the transition atomically so that
// an offer cannot be both withdrawn and accepted by concurrent requests.
func (s *MarketerJobRequestService) WithdrawOffer(ctx context.Context, actor Actor, offerID int32) (*domain.Offer, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if offerID < 1 {
		return nil, domain.ErrInvalidOffer
	}
	return s.repo.WithdrawOffer(ctx, actor.UserID, offerID)
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

// SubmitOffer creates a Pending offer for a request the Marketer accepted.
// A request may have only one active (Pending or Accepted) offer; a
// withdrawn or rejected offer does not block a new one.
func (s *MarketerJobRequestService) SubmitOffer(ctx context.Context, actor Actor, requestID int32, input SubmitOfferInput) (*domain.Offer, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if requestID < 1 {
		return nil, domain.ErrInvalidJobRequest
	}
	price, err := parsePrice(input.OfferedPrice)
	if err != nil {
		return nil, errors.Join(domain.ErrInvalidOffer, errors.New("offered_price must be a non-negative amount with at most two decimal places, up to 99999999.99"))
	}
	deliveryDate, err := time.Parse(time.DateOnly, strings.TrimSpace(input.DeliveryDate))
	if err != nil {
		return nil, errors.Join(domain.ErrInvalidOffer, errors.New("delivery_date must be a YYYY-MM-DD date"))
	}
	message, err := normalizeOfferMessage(input.Message)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if deliveryDate.Before(calendarDate(now)) {
		return nil, errors.Join(domain.ErrInvalidOffer, errors.New("delivery_date cannot be before today"))
	}

	request, err := s.repo.FindForMarketer(ctx, actor.UserID, requestID)
	if err != nil {
		return nil, err
	}
	if request.JobStatus != domain.JobStatusPending {
		return nil, domain.ErrJobNotPending
	}
	if request.Request.Status != domain.JobRequestStatusAccepted {
		return nil, domain.ErrJobRequestNotAccepted
	}
	if deadline := domain.EarliestSurveyDeadline(request.Surveys); deadline != nil && deliveryDate.After(calendarDate(*deadline)) {
		return nil, errors.Join(domain.ErrInvalidOffer, errors.New("delivery_date cannot be after the job deadline"))
	}

	offer := &domain.Offer{
		OfferedPrice: price,
		DeliveryDate: &deliveryDate,
		Message:      message,
		OfferStatus:  domain.OfferStatusPending,
		CreatedAt:    now.UTC(),
	}
	if err := s.repo.CreateOffer(ctx, actor.UserID, requestID, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListOffers returns the Marketer's offers for one request, newest first. The
// first item is the current offer.
func (s *MarketerJobRequestService) ListOffers(ctx context.Context, actor Actor, requestID int32) ([]domain.Offer, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if requestID < 1 {
		return nil, domain.ErrInvalidJobRequest
	}
	return s.repo.ListOffers(ctx, actor.UserID, requestID)
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

// calendarDate returns the Bangkok calendar date of t as midnight UTC, the
// same representation time.Parse gives a YYYY-MM-DD string.
func calendarDate(t time.Time) time.Time {
	year, month, day := t.In(offerDateLocation).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func normalizeOfferMessage(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	message := strings.TrimSpace(*value)
	if message == "" {
		return nil, nil
	}
	if len([]rune(message)) > domain.MaxOfferMessageLength {
		return nil, errors.Join(domain.ErrInvalidOffer, errors.New("message must be 300 characters or fewer"))
	}
	return &message, nil
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
