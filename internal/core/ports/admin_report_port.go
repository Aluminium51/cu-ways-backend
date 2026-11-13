package ports

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type AdminReportQuery struct {
	Page     int
	PageSize int
	Status   domain.JobStatus
}

type AdminReportPage[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

type AdminJob struct {
	JobID         int32                 `json:"job_id"`
	CreatorName   string                `json:"creator_name"`
	SurveyTitles  []string              `json:"survey_titles" gorm:"serializer:json"`
	JobStatus     domain.JobStatus      `json:"job_status"`
	Amount        *string               `json:"amount"`
	PaymentStatus *domain.PaymentStatus `json:"payment_status"`
	PaidAt        *time.Time            `json:"paid_at"`
}

type FlaggedMarketer struct {
	MarketerID      int32  `json:"marketer_id"`
	Name            string `json:"name"`
	FlaggedJobs     int64  `json:"flagged_jobs"`
	TotalValue      string `json:"total_value"`
	AttachmentCount int64  `json:"attachment_count"`
}

type AdminReportRepository interface {
	ListJobs(context.Context, AdminReportQuery) (AdminReportPage[AdminJob], error)
	ListFlaggedMarketers(context.Context, AdminReportQuery) (AdminReportPage[FlaggedMarketer], error)
}
