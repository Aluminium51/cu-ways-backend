package domain

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrCompletionPaymentRequired = errors.New("a paid payment matching the accepted offer is required")
	ErrCompletionProofRequired   = errors.New("delivery proof is required")
)

const PayoutStatusHeld = "Held"

type JobContact struct {
	Email  string  `json:"email"`
	Phone  *string `json:"phone"`
	LineID *string `json:"line_id"`
}

type JobSettlement struct {
	JobID           int32
	JobTitle        string
	JobStatus       JobStatus
	CompletedAt     *time.Time
	CreatorID       int32
	CreatorName     string
	CreatorContact  JobContact `gorm:"embedded;embeddedPrefix:creator_contact_"`
	MarketerID      int32
	MarketerName    string
	MarketerContact JobContact `gorm:"embedded;embeddedPrefix:marketer_contact_"`
	AcceptedPrice   decimal.Decimal
	Payment         *Payment `gorm:"-"`
}

// CanView checks participant access before reporting whether the job has an accepted offer.
func (s JobSettlement) CanView(userID int32, offerStatus OfferStatus) error {
	if userID != s.CreatorID && (userID != s.MarketerID || offerStatus != OfferStatusAccepted) {
		return ErrJobForbidden
	}
	if offerStatus != OfferStatusAccepted {
		return ErrAcceptedOfferRequired
	}
	return nil
}

func (s JobSettlement) PayoutStatus() string {
	if s.Payment == nil || s.Payment.PaymentStatus != PaymentStatusPaid {
		return ""
	}
	switch s.JobStatus {
	case JobStatusCompleted:
		return PayoutStatusReleased
	case JobStatusAccepted, JobStatusInProgress:
		return PayoutStatusHeld
	default:
		return ""
	}
}

func (s *JobSettlement) Complete(hasProof bool, completedAt time.Time) error {
	if s.JobStatus != JobStatusInProgress {
		return ErrJobNotInProgress
	}
	if s.Payment == nil || s.Payment.PaymentStatus != PaymentStatusPaid || !s.Payment.Amount.Equal(s.AcceptedPrice) {
		return ErrCompletionPaymentRequired
	}
	if !hasProof {
		return ErrCompletionProofRequired
	}
	s.JobStatus = JobStatusCompleted
	s.CompletedAt = &completedAt
	return nil
}
