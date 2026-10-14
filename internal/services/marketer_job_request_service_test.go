package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeMarketerJobRequestRepository struct {
	listMarketerID int32
	listStatus     *domain.JobRequestStatus
	respondCalls   int
	respondID      int32
	respondActor   int32
	response       ports.JobRequestResponse
	respondErr     error

	found         *ports.MarketerJobRequest
	findErr       error
	createCalls   int
	createdOffer  *domain.Offer
	createErr     error
	listOffersFor int32
}

func (f *fakeMarketerJobRequestRepository) FindForMarketer(_ context.Context, marketerID, requestID int32) (*ports.MarketerJobRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.found == nil {
		return nil, domain.ErrJobRequestNotFound
	}
	item := *f.found
	item.Request.JobRequestID = requestID
	item.Request.MarketerID = marketerID
	return &item, nil
}

func (f *fakeMarketerJobRequestRepository) CreateOffer(_ context.Context, marketerID, requestID int32, offer *domain.Offer) error {
	f.createCalls++
	if f.createErr != nil {
		return f.createErr
	}
	offer.OfferID = 31
	offer.JobID = 4
	offer.UserID = marketerID
	offer.JobRequestID = &requestID
	f.createdOffer = offer
	return nil
}

func (f *fakeMarketerJobRequestRepository) ListOffers(_ context.Context, _, requestID int32) ([]domain.Offer, error) {
	f.listOffersFor = requestID
	return []domain.Offer{}, nil
}

func (f *fakeMarketerJobRequestRepository) ListForMarketer(_ context.Context, marketerID int32, status *domain.JobRequestStatus) ([]ports.MarketerJobRequest, error) {
	f.listMarketerID = marketerID
	f.listStatus = status
	return []ports.MarketerJobRequest{}, nil
}

func (f *fakeMarketerJobRequestRepository) RespondToPending(_ context.Context, marketerID, requestID int32, response ports.JobRequestResponse) (*domain.JobRequest, error) {
	f.respondCalls++
	f.respondActor = marketerID
	f.respondID = requestID
	f.response = response
	if f.respondErr != nil {
		return nil, f.respondErr
	}
	respondedAt := response.RespondedAt
	return &domain.JobRequest{
		JobRequestID:  requestID,
		MarketerID:    marketerID,
		Status:        response.Status,
		RespondedAt:   &respondedAt,
		DeclineReason: response.DeclineReason,
		DeclineNote:   response.DeclineNote,
	}, nil
}

func newTestMarketerJobRequestService(repo *fakeMarketerJobRequestRepository, marketer bool) *MarketerJobRequestService {
	service := NewMarketerJobRequestService(repo, &fakeMembershipRepository{marketer: marketer})
	service.now = func() time.Time { return time.Date(2026, 10, 14, 5, 0, 0, 0, time.FixedZone("ICT", 7*60*60)) }
	return service
}

func TestMarketerJobRequestServiceRequiresMarketerMembership(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, false)
	ctx := context.Background()
	if _, err := service.List(ctx, Actor{UserID: 3}, nil); !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement on list, got %v", err)
	}
	if _, err := service.Accept(ctx, Actor{UserID: 3}, 1); !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement on accept, got %v", err)
	}
	if _, err := service.Decline(ctx, Actor{UserID: 3}, 1, DeclineJobRequestInput{}); !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement on decline, got %v", err)
	}
	if repo.respondCalls != 0 || repo.listMarketerID != 0 {
		t.Fatal("non-marketer reached persistence")
	}
}

func TestMarketerJobRequestServiceListsOnlyTheActorsRequests(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	status := domain.JobRequestStatusPending
	if _, err := service.List(context.Background(), Actor{UserID: 9}, &status); err != nil {
		t.Fatal(err)
	}
	if repo.listMarketerID != 9 || repo.listStatus == nil || *repo.listStatus != domain.JobRequestStatusPending {
		t.Fatalf("unexpected list query: marketer=%d status=%v", repo.listMarketerID, repo.listStatus)
	}
	invalid := domain.JobRequestStatus("Unknown")
	if _, err := service.List(context.Background(), Actor{UserID: 9}, &invalid); !errors.Is(err, domain.ErrInvalidJobRequest) {
		t.Fatalf("expected invalid status rejection, got %v", err)
	}
}

func TestMarketerJobRequestServiceAcceptRecordsUTCResponseTime(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	request, err := service.Accept(context.Background(), Actor{UserID: 9}, 21)
	if err != nil {
		t.Fatal(err)
	}
	if repo.respondActor != 9 || repo.respondID != 21 || repo.response.Status != domain.JobRequestStatusAccepted {
		t.Fatalf("unexpected accept call: %+v", repo)
	}
	if repo.response.RespondedAt.Location() != time.UTC || !repo.response.RespondedAt.Equal(time.Date(2026, 10, 13, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected UTC response time, got %v", repo.response.RespondedAt)
	}
	if repo.response.DeclineReason != nil || repo.response.DeclineNote != nil {
		t.Fatal("accept must not carry decline details")
	}
	if request.Status != domain.JobRequestStatusAccepted {
		t.Fatalf("unexpected result: %+v", request)
	}
}

func TestMarketerJobRequestServiceDeclineNormalizesOptionalDetails(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	reason := "  Budget too low "
	note := "  Already booked this month.  "
	if _, err := service.Decline(context.Background(), Actor{UserID: 9}, 21, DeclineJobRequestInput{Reason: &reason, Note: &note}); err != nil {
		t.Fatal(err)
	}
	if repo.response.Status != domain.JobRequestStatusDeclined {
		t.Fatalf("expected Declined, got %s", repo.response.Status)
	}
	if repo.response.DeclineReason == nil || *repo.response.DeclineReason != "Budget too low" {
		t.Fatalf("unexpected reason: %v", repo.response.DeclineReason)
	}
	if repo.response.DeclineNote == nil || *repo.response.DeclineNote != "Already booked this month." {
		t.Fatalf("unexpected note: %v", repo.response.DeclineNote)
	}

	blank := "   "
	if _, err := service.Decline(context.Background(), Actor{UserID: 9}, 22, DeclineJobRequestInput{Reason: &blank, Note: &blank}); err != nil {
		t.Fatal(err)
	}
	if repo.response.DeclineReason != nil || repo.response.DeclineNote != nil {
		t.Fatal("blank decline details must be stored as NULL")
	}
}

func TestMarketerJobRequestServiceRejectsInvalidDeclineDetailsBeforePersistence(t *testing.T) {
	unknownReason := "Too far away"
	longNote := strings.Repeat("ก", domain.MaxJobRequestDeclineNoteLength+1)
	maxNote := strings.Repeat("ก", domain.MaxJobRequestDeclineNoteLength)
	for name, input := range map[string]DeclineJobRequestInput{
		"unknown_reason": {Reason: &unknownReason},
		"long_note":      {Note: &longNote},
	} {
		repo := &fakeMarketerJobRequestRepository{}
		service := newTestMarketerJobRequestService(repo, true)
		if _, err := service.Decline(context.Background(), Actor{UserID: 9}, 21, input); !errors.Is(err, domain.ErrInvalidJobRequest) {
			t.Fatalf("%s: expected invalid job request, got %v", name, err)
		}
		if repo.respondCalls != 0 {
			t.Fatalf("%s: invalid decline reached persistence", name)
		}
	}
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	if _, err := service.Decline(context.Background(), Actor{UserID: 9}, 21, DeclineJobRequestInput{Note: &maxNote}); err != nil {
		t.Fatalf("a %d-character note must be accepted: %v", domain.MaxJobRequestDeclineNoteLength, err)
	}
}

func acceptedRequestWithDeadline(deadline *time.Time) *ports.MarketerJobRequest {
	return &ports.MarketerJobRequest{
		Request: domain.JobRequest{JobID: 4, Status: domain.JobRequestStatusAccepted},
		Surveys: []domain.Survey{{SurveyID: 5, Deadline: deadline}},
	}
}

func TestMarketerJobRequestServiceSubmitsPendingOfferForAcceptedRequest(t *testing.T) {
	deadline := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	repo := &fakeMarketerJobRequestRepository{found: acceptedRequestWithDeadline(&deadline)}
	service := newTestMarketerJobRequestService(repo, true)
	message := "  Faculty LINE groups and a campus booth.  "
	offer, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{
		OfferedPrice: "9000.5",
		DeliveryDate: "2026-10-28",
		Message:      &message,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.createCalls != 1 || offer.OfferStatus != domain.OfferStatusPending || offer.UserID != 9 || offer.JobRequestID == nil || *offer.JobRequestID != 21 {
		t.Fatalf("unexpected offer: %+v", offer)
	}
	if offer.OfferedPrice.StringFixed(2) != "9000.50" {
		t.Fatalf("unexpected price: %s", offer.OfferedPrice)
	}
	if offer.DeliveryDate == nil || offer.DeliveryDate.Format(time.DateOnly) != "2026-10-28" {
		t.Fatalf("unexpected delivery date: %v", offer.DeliveryDate)
	}
	if offer.Message == nil || *offer.Message != "Faculty LINE groups and a campus booth." {
		t.Fatalf("message must be trimmed, got %v", offer.Message)
	}
	if offer.CreatedAt.Location() != time.UTC {
		t.Fatal("created_at must be UTC")
	}
}

func TestMarketerJobRequestServiceAcceptsZeroPriceAndJobsWithoutDeadline(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{found: acceptedRequestWithDeadline(nil)}
	service := newTestMarketerJobRequestService(repo, true)
	if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{OfferedPrice: "0", DeliveryDate: "2027-03-01"}); err != nil {
		t.Fatalf("zero price without a deadline must be accepted: %v", err)
	}
}

func TestMarketerJobRequestServiceRejectsInvalidOfferFieldsBeforePersistence(t *testing.T) {
	deadline := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	longMessage := strings.Repeat("a", domain.MaxOfferMessageLength+1)
	for name, input := range map[string]SubmitOfferInput{
		"negative_price":   {OfferedPrice: "-1", DeliveryDate: "2026-10-20"},
		"three_decimals":   {OfferedPrice: "10.001", DeliveryDate: "2026-10-20"},
		"too_large":        {OfferedPrice: "100000000", DeliveryDate: "2026-10-20"},
		"not_a_number":     {OfferedPrice: "abc", DeliveryDate: "2026-10-20"},
		"missing_price":    {DeliveryDate: "2026-10-20"},
		"bad_date":         {OfferedPrice: "10", DeliveryDate: "20/10/2026"},
		"impossible_date":  {OfferedPrice: "10", DeliveryDate: "2026-02-30"},
		"missing_delivery": {OfferedPrice: "10"},
		"before_today":     {OfferedPrice: "10", DeliveryDate: "2026-10-13"},
		"after_deadline":   {OfferedPrice: "10", DeliveryDate: "2026-10-29"},
		"message_too_long": {OfferedPrice: "10", DeliveryDate: "2026-10-20", Message: &longMessage},
	} {
		repo := &fakeMarketerJobRequestRepository{found: acceptedRequestWithDeadline(&deadline)}
		service := newTestMarketerJobRequestService(repo, true)
		if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, input); !errors.Is(err, domain.ErrInvalidOffer) {
			t.Fatalf("%s: expected invalid offer, got %v", name, err)
		}
		if repo.createCalls != 0 {
			t.Fatalf("%s: invalid offer reached persistence", name)
		}
	}
}

func TestMarketerJobRequestServiceUsesBangkokCalendarForToday(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{found: acceptedRequestWithDeadline(nil)}
	service := newTestMarketerJobRequestService(repo, true)
	// The test clock is 2026-10-14 05:00 in Bangkok, which is still
	// 2026-10-13 in UTC. The Bangkok date is today.
	if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{OfferedPrice: "10", DeliveryDate: "2026-10-14"}); err != nil {
		t.Fatalf("the Bangkok date must count as today: %v", err)
	}
}

func TestMarketerJobRequestServiceRequiresAcceptedRequestForOffer(t *testing.T) {
	for _, status := range []domain.JobRequestStatus{domain.JobRequestStatusPending, domain.JobRequestStatusDeclined, domain.JobRequestStatusClosed} {
		found := acceptedRequestWithDeadline(nil)
		found.Request.Status = status
		repo := &fakeMarketerJobRequestRepository{found: found}
		service := newTestMarketerJobRequestService(repo, true)
		if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{OfferedPrice: "10", DeliveryDate: "2026-10-20"}); !errors.Is(err, domain.ErrJobRequestNotAccepted) {
			t.Fatalf("%s: expected not accepted, got %v", status, err)
		}
		if repo.createCalls != 0 {
			t.Fatalf("%s: offer reached persistence", status)
		}
	}
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{OfferedPrice: "10", DeliveryDate: "2026-10-20"}); !errors.Is(err, domain.ErrJobRequestNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestMarketerJobRequestServicePropagatesDuplicateOffer(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{found: acceptedRequestWithDeadline(nil), createErr: domain.ErrOfferAlreadyExists}
	service := newTestMarketerJobRequestService(repo, true)
	if _, err := service.SubmitOffer(context.Background(), Actor{UserID: 9}, 21, SubmitOfferInput{OfferedPrice: "10", DeliveryDate: "2026-10-20"}); !errors.Is(err, domain.ErrOfferAlreadyExists) {
		t.Fatalf("expected duplicate offer error, got %v", err)
	}
}

func TestMarketerJobRequestServiceListOffersRequiresMarketer(t *testing.T) {
	repo := &fakeMarketerJobRequestRepository{}
	if _, err := newTestMarketerJobRequestService(repo, false).ListOffers(context.Background(), Actor{UserID: 9}, 21); !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement, got %v", err)
	}
	if _, err := newTestMarketerJobRequestService(repo, true).ListOffers(context.Background(), Actor{UserID: 9}, 21); err != nil || repo.listOffersFor != 21 {
		t.Fatalf("expected offers listed for request 21, got err=%v request=%d", err, repo.listOffersFor)
	}
}

func TestMarketerJobRequestServicePropagatesStateErrors(t *testing.T) {
	for _, repoErr := range []error{domain.ErrJobRequestNotFound, domain.ErrJobRequestNotPending} {
		repo := &fakeMarketerJobRequestRepository{respondErr: repoErr}
		service := newTestMarketerJobRequestService(repo, true)
		if _, err := service.Accept(context.Background(), Actor{UserID: 9}, 21); !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	}
	repo := &fakeMarketerJobRequestRepository{}
	service := newTestMarketerJobRequestService(repo, true)
	if _, err := service.Accept(context.Background(), Actor{UserID: 9}, 0); !errors.Is(err, domain.ErrInvalidJobRequest) || repo.respondCalls != 0 {
		t.Fatalf("expected invalid id rejection before persistence, got %v", err)
	}
}
