package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
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
		job, err := lockJob(tx, jobID)
		if err != nil {
			return err
		}
		// The (job_id, accepted_offer_id) foreign key guarantees the accepted
		// offer exists and belongs to this job.
		var accepted *domain.Offer
		if job.AcceptedOfferID != nil {
			var offer domain.Offer
			if err := tx.Where("offer_id = ? AND job_id = ?", *job.AcceptedOfferID, job.JobID).First(&offer).Error; err != nil {
				return err
			}
			accepted = &offer
		}
		if err := job.Start(marketerID, accepted); err != nil {
			return err
		}
		if err := tx.Model(job).Update("job_status", job.JobStatus).Error; err != nil {
			return err
		}
		result = ports.StartedJob{Job: *job, Offer: *accepted}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
