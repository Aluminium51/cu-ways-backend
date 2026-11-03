package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionRole selects which side of a completed Job a user is viewing.
type TransactionRole string

const (
	TransactionRoleCreator  TransactionRole = "creator"
	TransactionRoleMarketer TransactionRole = "marketer"
)

// IsValid reports whether the role is one the transaction history accepts.
// An empty role means "every Job the user is involved in".
func (r TransactionRole) IsValid() bool {
	return r == "" || r == TransactionRoleCreator || r == TransactionRoleMarketer
}

// PayoutStatusReleased is reported for every Completed Job in the history.
// The payout state is derived from the Job and Payment status; it is not
// stored, because the platform records payments and does not move money.
const PayoutStatusReleased = "Released"

// Transaction is the financial breakdown of one Completed Job whose accepted
// offer has a Paid payment.
type Transaction struct {
	JobID         int32           `gorm:"column:job_id"`
	JobTitle      string          `gorm:"column:job_title"`
	CreatorID     int32           `gorm:"column:creator_id"`
	CreatorName   string          `gorm:"column:creator_name"`
	MarketerID    int32           `gorm:"column:marketer_id"`
	MarketerName  string          `gorm:"column:marketer_name"`
	AcceptedPrice decimal.Decimal `gorm:"column:accepted_price"`
	PaymentMethod PaymentMethod   `gorm:"column:payment_method"`
	PaidAt        *time.Time      `gorm:"column:paid_at"`
	PayoutStatus  string          `gorm:"-"`
}
