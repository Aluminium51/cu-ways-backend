package postgres

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MarketerJobRepository struct {
	db *gorm.DB
}

var _ ports.MarketerJobRepository = (*MarketerJobRepository)(nil)

func NewMarketerJobRepository(db *gorm.DB) *MarketerJobRepository {
	return &MarketerJobRepository{db: db}
}

func (r *MarketerJobRepository) StartJob(ctx context.Context, marketerID, jobID int32) (*ports.StartedJob, error) {
	var result ports.StartedJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("job_id = ?", jobID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrJobNotFound
			}
			return err
		}
		if job.AcceptedOfferID == nil {
			return domain.ErrJobForbidden
		}
		// Look the offer up by ID alone so that an accepted_offer_id pointing at
		// another job's offer is reported as a mismatch rather than hidden.
		var offer domain.Offer
		if err := tx.Where("offer_id = ?", *job.AcceptedOfferID).First(&offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrAcceptedOfferMismatch
			}
			return err
		}
		if offer.UserID != marketerID {
			return domain.ErrJobForbidden
		}
		if offer.JobID != job.JobID || offer.OfferStatus != domain.OfferStatusAccepted {
			return domain.ErrAcceptedOfferMismatch
		}
		if job.JobStatus != domain.JobStatusAccepted {
			return domain.ErrJobNotAccepted
		}
		if err := tx.Model(&job).Update("job_status", domain.JobStatusInProgress).Error; err != nil {
			return err
		}
		job.JobStatus = domain.JobStatusInProgress
		result = ports.StartedJob{Job: job, Offer: offer}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
