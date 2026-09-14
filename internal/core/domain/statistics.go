package domain

import "github.com/shopspring/decimal"

// MarketerStatistics summarizes completed work for one marketer.
type MarketerStatistics struct {
	TotalCompletedJobs int64
	AverageRating      *float64
	TotalEarnings      decimal.Decimal
}
