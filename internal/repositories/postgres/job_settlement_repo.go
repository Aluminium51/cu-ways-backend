package postgres

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type JobSettlementRepository struct{ db *gorm.DB }

var _ ports.JobSettlementRepository = (*JobSettlementRepository)(nil)

func NewJobSettlementRepository(db *gorm.DB) *JobSettlementRepository {
	return &JobSettlementRepository{db: db}
}

// Read the payment and its completion state in one statement so a concurrent
// completion cannot produce a Held response with a completed timestamp.
func readJobSettlement(db *gorm.DB, userID, jobID int32) (*domain.JobSettlement, error) {
	type settlementRow struct {
		domain.JobSettlement `gorm:"embedded"`
		domain.Payment       `gorm:"embedded;embeddedPrefix:paid_"`
		OfferStatus          domain.OfferStatus
	}
	var row settlementRow
	result := db.Table("jobs AS j").
		Select(`j.job_id, COALESCE(j.title, '') AS job_title, j.job_status, j.completed_at,
j.user_id AS creator_id, cu.name AS creator_name, o.user_id AS marketer_id, mu.name AS marketer_name,
CASE WHEN cu.deleted_at IS NULL THEN cu.email ELSE '' END AS creator_contact_email,
CASE WHEN cu.deleted_at IS NULL THEN cu.phone END AS creator_contact_phone,
CASE WHEN cu.deleted_at IS NULL THEN cu.line_id END AS creator_contact_line_id,
CASE WHEN mu.deleted_at IS NULL THEN COALESCE(mu.email, '') ELSE '' END AS marketer_contact_email,
CASE WHEN mu.deleted_at IS NULL THEN mu.phone END AS marketer_contact_phone,
CASE WHEN mu.deleted_at IS NULL THEN mu.line_id END AS marketer_contact_line_id,
o.offered_price AS accepted_price, o.offer_status,
p.payment_id AS paid_payment_id, p.job_id AS paid_job_id, p.amount AS paid_amount,
p.method AS paid_method, p.payment_status AS paid_payment_status, p.created_at AS paid_created_at, p.paid_at AS paid_paid_at`).
		Joins("JOIN users AS cu ON cu.user_id = j.user_id").
		Joins("LEFT JOIN offers AS o ON o.job_id = j.job_id AND o.offer_id = j.accepted_offer_id").
		Joins("LEFT JOIN users AS mu ON mu.user_id = o.user_id").
		Joins(`LEFT JOIN LATERAL (
SELECT * FROM payments WHERE job_id = j.job_id AND payment_status = ?
ORDER BY paid_at DESC NULLS LAST, payment_id DESC LIMIT 1
) AS p ON TRUE`, domain.PaymentStatusPaid).
		Where("j.job_id = ?", jobID).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrJobNotFound
	}
	if err := row.CanView(userID, row.OfferStatus); err != nil {
		return nil, err
	}
	if row.PaymentID != 0 {
		row.JobSettlement.Payment = &row.Payment
	}
	return &row.JobSettlement, nil
}

func (r *JobSettlementRepository) GetPayment(ctx context.Context, userID, jobID int32) (*domain.JobSettlement, error) {
	return readJobSettlement(r.db.WithContext(ctx), userID, jobID)
}

func (r *JobSettlementRepository) Complete(ctx context.Context, creatorID, jobID int32) (*domain.JobSettlement, error) {
	var saved *domain.JobSettlement
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		saved, err = readJobSettlement(tx, creatorID, jobID)
		if err != nil || job.JobStatus == domain.JobStatusCompleted {
			return err
		}
		var proofs int64
		if err := tx.Model(&domain.Attachment{}).Where("job_id = ? AND attachment_type = ?", jobID, domain.AttachmentTypeProof).Count(&proofs).Error; err != nil {
			return err
		}
		if err := saved.Complete(proofs > 0, time.Now().UTC().Truncate(time.Microsecond)); err != nil {
			return err
		}
		return tx.Model(job).Updates(map[string]any{"job_status": saved.JobStatus, "completed_at": saved.CompletedAt}).Error
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}
