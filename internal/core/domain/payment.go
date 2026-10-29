package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "Pending"
	PaymentStatusPaid     PaymentStatus = "Paid"
	PaymentStatusFailed   PaymentStatus = "Failed"
	PaymentStatusRefunded PaymentStatus = "Refunded"
)

type PaymentMethod string

const (
	PaymentMethodBankTransfer PaymentMethod = "Bank Transfer"
	PaymentMethodCreditCard   PaymentMethod = "Credit Card"
	PaymentMethodPromptPay    PaymentMethod = "PromptPay"
	PaymentMethodOther        PaymentMethod = "Other"
)

type Payment struct {
	PaymentID     int32           `gorm:"column:payment_id;primaryKey;autoIncrement"`
	JobID         int32           `gorm:"column:job_id;not null"`
	Amount        decimal.Decimal `gorm:"column:amount;type:decimal(10,2);not null"`
	Method        PaymentMethod   `gorm:"column:method;type:varchar(30);not null"`
	PaymentStatus PaymentStatus   `gorm:"column:payment_status;type:varchar(20);not null"`
	PaidAt        *time.Time      `gorm:"column:paid_at;type:timestamp"`
	CreatedAt     *time.Time      `gorm:"column:created_at;type:timestamp"`

	Job *Job `gorm:"foreignKey:JobID;references:JobID"`
}

func (Payment) TableName() string { return "payments" }

func (m PaymentMethod) IsSupported() bool {
	return m == PaymentMethodPromptPay || m == PaymentMethodBankTransfer || m == PaymentMethodCreditCard
}

// ValidateForJob evaluates the payment against the Job and offer snapshot
// held by the persistence transaction.
func (p Payment) ValidateForJob(job Job, offer *Offer, hasActivePayment bool) error {
	switch job.JobStatus {
	case JobStatusAccepted, JobStatusInProgress, JobStatusCompleted:
	default:
		return ErrJobNotPayable
	}
	if job.AcceptedOfferID == nil || offer == nil || offer.JobID != job.JobID || offer.OfferID != *job.AcceptedOfferID || offer.OfferStatus != OfferStatusAccepted {
		return ErrAcceptedOfferRequired
	}
	if !p.Amount.Equal(offer.OfferedPrice) {
		return ErrPaymentAmountMismatch
	}
	if hasActivePayment {
		return ErrPaymentAlreadyExists
	}
	return nil
}
