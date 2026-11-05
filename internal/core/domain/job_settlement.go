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

type JobSettlement struct {
	JobID         int32
	JobTitle      string
	JobStatus     JobStatus
	CompletedAt   *time.Time
	CreatorID     int32
	CreatorName   string
	MarketerID    int32
	MarketerName  string
	AcceptedPrice decimal.Decimal
	Payment       *Payment `gorm:"-"`
}

func (s JobSettlement) PayoutStatus() string {
	if s.Payment == nil {
		return ""
	}
	if s.JobStatus == JobStatusCompleted {
		return PayoutStatusReleased
	}
	return PayoutStatusHeld
}

func (s *JobSettlement) Complete(hasProof bool, completedAt time.Time) error {
	if s.JobStatus != JobStatusInProgress {
		return ErrJobNotInProgress
	}
	if s.Payment == nil || !s.Payment.Amount.Equal(s.AcceptedPrice) {
		return ErrCompletionPaymentRequired
	}
	if !hasProof {
		return ErrCompletionProofRequired
	}
	s.JobStatus = JobStatusCompleted
	s.CompletedAt = &completedAt
	return nil
}
