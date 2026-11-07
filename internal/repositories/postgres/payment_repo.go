package postgres

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentRepository struct{ db *gorm.DB }

var _ ports.PaymentRepository = (*PaymentRepository)(nil)

func NewPaymentRepository(db *gorm.DB) *PaymentRepository { return &PaymentRepository{db: db} }

func (r *PaymentRepository) CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, payment *domain.Payment) (*domain.Payment, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		var offer *domain.Offer
		if job.AcceptedOfferID != nil {
			var accepted domain.Offer
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("job_id = ? AND offer_id = ?", jobID, *job.AcceptedOfferID).First(&accepted).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				offer = &accepted
			}
		}
		var active int64
		if err := tx.Model(&domain.Payment{}).Where("job_id = ? AND payment_status IN ?", jobID, []domain.PaymentStatus{domain.PaymentStatusPending, domain.PaymentStatusPaid}).Count(&active).Error; err != nil {
			return err
		}
		if err := payment.ValidateForJob(*job, offer, active > 0); err != nil {
			return err
		}
		payment.JobID = jobID
		return tx.Create(payment).Error
	})
	if err != nil {
		return nil, err
	}
	return payment, nil
}

// FindActiveForCreatorJob returns the owned Job's Pending or Paid payment, the
// same set that blocks a new payment. Creation allows only one, so the newest
// is returned if legacy data holds more. Failed and Refunded history is ignored.
func (r *PaymentRepository) FindActiveForCreatorJob(ctx context.Context, creatorID, jobID int32) (*domain.Payment, error) {
	db := r.db.WithContext(ctx)
	if _, err := findOwnedJob(db, creatorID, jobID); err != nil {
		return nil, err
	}
	var payments []domain.Payment
	if err := db.Where("job_id = ? AND payment_status IN ?", jobID, []domain.PaymentStatus{domain.PaymentStatusPending, domain.PaymentStatusPaid}).
		Order("payment_id DESC").Limit(1).Find(&payments).Error; err != nil {
		return nil, err
	}
	if len(payments) == 0 {
		return nil, nil
	}
	return &payments[0], nil
}
