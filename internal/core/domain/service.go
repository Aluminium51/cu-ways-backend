package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Service struct {
	ServiceID   int32           `gorm:"column:service_id;primaryKey;autoIncrement"`
	UserID      int32           `gorm:"column:user_id;not null"`
	ServiceType string          `gorm:"column:service_type;type:varchar(100);not null"`
	ScopeText   *string         `gorm:"column:scope_text;type:text"`
	Price       decimal.Decimal `gorm:"column:price;type:decimal(10,2);not null"`
	CreatedAt   time.Time       `gorm:"column:created_at;type:timestamp;not null"`

	Marketer *Marketer `gorm:"foreignKey:UserID;references:UserID"`
}

func (Service) TableName() string { return "services" }
