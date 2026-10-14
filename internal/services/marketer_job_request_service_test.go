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
