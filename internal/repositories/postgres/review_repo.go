package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReviewRepository struct {
	db *gorm.DB
}

var _ ports.ReviewRepository = (*ReviewRepository)(nil)

func NewReviewRepository(db *gorm.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, review *domain.Review) (*domain.Review, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		if job.JobStatus != domain.JobStatusCompleted {
			return domain.ErrJobNotCompleted
		}
		review.JobID = jobID
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true}).Create(review)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrReviewAlreadyExists
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return review, nil
}
