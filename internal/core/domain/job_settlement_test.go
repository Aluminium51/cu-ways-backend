package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestJobSettlementComplete(t *testing.T) {
	completedAt := time.Date(2026, time.November, 7, 8, 0, 0, 0, time.UTC)
	price := decimal.NewFromInt(100)
	paid := &Payment{PaymentStatus: PaymentStatusPaid, Amount: price}
	for _, tt := range []struct {
		name    string
		status  JobStatus
		payment *Payment
		proof   bool
		want    error
	}{
		{"ready", JobStatusInProgress, paid, true, nil},
		{"not started", JobStatusAccepted, paid, true, ErrJobNotInProgress},
		{"already completed", JobStatusCompleted, paid, true, ErrJobNotInProgress},
		{"no payment", JobStatusInProgress, nil, true, ErrCompletionPaymentRequired},
		{"pending payment", JobStatusInProgress, &Payment{PaymentStatus: PaymentStatusPending, Amount: price}, true, ErrCompletionPaymentRequired},
		{"wrong amount", JobStatusInProgress, &Payment{PaymentStatus: PaymentStatusPaid, Amount: decimal.NewFromInt(99)}, true, ErrCompletionPaymentRequired},
		{"no proof", JobStatusInProgress, paid, false, ErrCompletionProofRequired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job := JobSettlement{JobStatus: tt.status, AcceptedPrice: price, Payment: tt.payment}
			if err := job.Complete(tt.proof, completedAt); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if tt.want != nil {
				if job.JobStatus != tt.status || job.CompletedAt != nil {
					t.Fatalf("failed completion changed the job: %+v", job)
				}
				return
			}
			if job.JobStatus != JobStatusCompleted || job.CompletedAt == nil || !job.CompletedAt.Equal(completedAt) || job.PayoutStatus() != PayoutStatusReleased {
				t.Fatalf("unexpected completed job: %+v", job)
			}
		})
	}
}

func TestJobSettlementPayoutStatus(t *testing.T) {
	paid := &Payment{PaymentStatus: PaymentStatusPaid}
	for _, tt := range []struct {
		name    string
		status  JobStatus
		payment *Payment
		want    string
	}{
		{"accepted", JobStatusAccepted, paid, PayoutStatusHeld},
		{"in progress", JobStatusInProgress, paid, PayoutStatusHeld},
		{"completed", JobStatusCompleted, paid, PayoutStatusReleased},
		{"cancelled", JobStatusCancelled, paid, ""},
		{"pending job", JobStatusPending, paid, ""},
		{"pending payment", JobStatusInProgress, &Payment{PaymentStatus: PaymentStatusPending}, ""},
		{"missing payment", JobStatusCompleted, nil, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			job := JobSettlement{JobStatus: tt.status, Payment: tt.payment}
			if got := job.PayoutStatus(); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJobSettlementCanView(t *testing.T) {
	job := JobSettlement{CreatorID: 1, MarketerID: 2}
	for _, tt := range []struct {
		userID      int32
		offerStatus OfferStatus
		want        error
	}{
		{1, OfferStatusAccepted, nil},
		{2, OfferStatusAccepted, nil},
		{3, OfferStatusAccepted, ErrJobForbidden},
		{1, "", ErrAcceptedOfferRequired},
		{2, OfferStatusRejected, ErrJobForbidden},
	} {
		if err := job.CanView(tt.userID, tt.offerStatus); !errors.Is(err, tt.want) {
			t.Errorf("user %d, offer %q: got %v, want %v", tt.userID, tt.offerStatus, err, tt.want)
		}
	}
}
