package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentEligibility(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            JobStatus
		offerStatus       OfferStatus
		offerJob, offerID int32
		amount            string
		active, missing   bool
		want              error
	}{
		{name: "accepted", status: JobStatusAccepted, offerStatus: OfferStatusAccepted, offerJob: 12, offerID: 7, amount: "100"},
		{name: "in_progress", status: JobStatusInProgress, offerStatus: OfferStatusAccepted, offerJob: 12, offerID: 7, amount: "100.00"},
		{name: "completed", status: JobStatusCompleted, offerStatus: OfferStatusAccepted, offerJob: 12, offerID: 7, amount: "100"},
		{name: "pending_job", status: JobStatusPending, want: ErrJobNotPayable},
		{name: "cancelled_job", status: JobStatusCancelled, want: ErrJobNotPayable},
		{name: "missing_offer", status: JobStatusAccepted, missing: true, want: ErrAcceptedOfferRequired},
		{name: "foreign_offer", status: JobStatusAccepted, offerJob: 13, offerID: 7, offerStatus: OfferStatusAccepted, want: ErrAcceptedOfferRequired},
		{name: "wrong_offer", status: JobStatusAccepted, offerJob: 12, offerID: 8, offerStatus: OfferStatusAccepted, want: ErrAcceptedOfferRequired},
		{name: "unaccepted_offer", status: JobStatusAccepted, offerJob: 12, offerID: 7, offerStatus: OfferStatusPending, want: ErrAcceptedOfferRequired},
		{name: "underpayment", status: JobStatusAccepted, offerJob: 12, offerID: 7, offerStatus: OfferStatusAccepted, amount: "99.99", want: ErrPaymentAmountMismatch},
		{name: "overpayment", status: JobStatusAccepted, offerJob: 12, offerID: 7, offerStatus: OfferStatusAccepted, amount: "100.01", want: ErrPaymentAmountMismatch},
		{name: "active_payment", status: JobStatusAccepted, offerJob: 12, offerID: 7, offerStatus: OfferStatusAccepted, amount: "100", active: true, want: ErrPaymentAlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := int32(7)
			job := Job{JobID: 12, JobStatus: tc.status, AcceptedOfferID: &id}
			offer := &Offer{OfferID: tc.offerID, JobID: tc.offerJob, OfferStatus: tc.offerStatus, OfferedPrice: decimal.RequireFromString("100.00")}
			if tc.missing {
				offer = nil
				job.AcceptedOfferID = nil
			}
			amount := decimal.Zero
			if tc.amount != "" {
				amount = decimal.RequireFromString(tc.amount)
			}
			p := Payment{Amount: amount}
			if err := p.ValidateForJob(job, offer, tc.active); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
	id := int32(7)
	if err := (&Payment{Amount: decimal.Zero}).ValidateForJob(Job{JobID: 12, JobStatus: JobStatusAccepted, AcceptedOfferID: &id}, &Offer{JobID: 12, OfferID: 7, OfferStatus: OfferStatusAccepted, OfferedPrice: decimal.Zero}, false); err != nil {
		t.Fatal(err)
	}
}
