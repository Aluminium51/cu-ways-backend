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
	JobID         int32           `gorm:"column:job_id;not null;uniqueIndex"`
	Amount        decimal.Decimal `gorm:"column:amount;type:decimal(10,2);not null"`
	Method        PaymentMethod   `gorm:"column:method;type:varchar(30);not null"`
	PaymentStatus PaymentStatus   `gorm:"column:payment_status;type:varchar(20);not null"`
	PaidAt        *time.Time      `gorm:"column:paid_at;type:timestamp"`

	Job *Job `gorm:"foreignKey:JobID;references:JobID"`
}

func (Payment) TableName() string { return "payments" }
