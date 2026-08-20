package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type OfferStatus string

const (
	OfferStatusPending   OfferStatus = "Pending"
	OfferStatusAccepted  OfferStatus = "Accepted"
	OfferStatusRejected  OfferStatus = "Rejected"
	OfferStatusWithdrawn OfferStatus = "Withdrawn"
)

type Offer struct {
	OfferID      int32           `gorm:"column:offer_id;primaryKey;autoIncrement"`
	JobID        int32           `gorm:"column:job_id;not null;uniqueIndex:idx_offers_job_offer"`
	UserID       int32           `gorm:"column:user_id;not null"`
	OfferedPrice decimal.Decimal `gorm:"column:offered_price;type:decimal(10,2);not null"`
	OfferStatus  OfferStatus     `gorm:"column:offer_status;type:varchar(20);not null"`
	CreatedAt    time.Time       `gorm:"column:created_at;type:timestamp;not null"`

	Job      *Job      `gorm:"foreignKey:JobID;references:JobID"`
	Marketer *Marketer `gorm:"foreignKey:UserID;references:UserID"`
}

func (Offer) TableName() string { return "offers" }
