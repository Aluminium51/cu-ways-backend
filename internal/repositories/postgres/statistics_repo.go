package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type MarketerStatisticsRepository struct {
	db *gorm.DB
}

var _ ports.MarketerStatisticsRepository = (*MarketerStatisticsRepository)(nil)

func NewMarketerStatisticsRepository(db *gorm.DB) *MarketerStatisticsRepository {
	return &MarketerStatisticsRepository{db: db}
}

func (r *MarketerStatisticsRepository) GetByMarketer(ctx context.Context, userID int32) (domain.MarketerStatistics, error) {
	var result domain.MarketerStatistics
	err := r.db.WithContext(ctx).Raw(`
SELECT
    COUNT(*) AS total_completed_jobs,
    AVG(review.rating)::float8 AS average_rating,
    COALESCE(SUM(COALESCE(paid.total, 0)), 0) AS total_earnings
FROM jobs AS job
JOIN offers AS accepted_offer
  ON accepted_offer.job_id = job.job_id
 AND accepted_offer.offer_id = job.accepted_offer_id
LEFT JOIN reviews AS review
  ON review.job_id = job.job_id
LEFT JOIN (
    SELECT payment.job_id, SUM(payment.amount) AS total
    FROM payments AS payment
    WHERE payment.payment_status = ?
    GROUP BY payment.job_id
) AS paid
  ON paid.job_id = job.job_id
WHERE job.job_status = ?
  AND accepted_offer.user_id = ?`,
		domain.PaymentStatusPaid,
		domain.JobStatusCompleted,
		userID,
	).Scan(&result).Error
	return result, err
}
