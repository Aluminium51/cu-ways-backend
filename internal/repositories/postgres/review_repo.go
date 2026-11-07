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

// verifiedMarketerReviews narrows reviews to the ones that count on a public
// profile: a review whose Job is Completed and whose accepted Offer belongs to
// the marketer, with a rating in the valid 1-5 range. The marketer filter comes
// from the path, and only verified rows are ever joined, so an unrelated or
// unfinished Job can never surface here.
func (r *ReviewRepository) verifiedMarketerReviews(ctx context.Context, marketerID int32) *gorm.DB {
	return r.db.WithContext(ctx).Table("reviews AS review").
		Joins("JOIN jobs AS job ON job.job_id = review.job_id").
		Joins("JOIN offers AS accepted_offer ON accepted_offer.job_id = job.job_id AND accepted_offer.offer_id = job.accepted_offer_id").
		Where("job.job_status = ?", domain.JobStatusCompleted).
		Where("accepted_offer.user_id = ?", marketerID).
		Where("review.rating BETWEEN 1 AND 5")
}

func (r *ReviewRepository) ListForMarketer(ctx context.Context, query ports.MarketerReviewQuery) (ports.MarketerReviewPage, error) {
	page := ports.MarketerReviewPage{Page: query.Page, PageSize: query.PageSize, Items: []domain.Review{}}

	var aggregate struct {
		Total         int64
		AverageRating *float64
	}
	// AVG returns NULL with no qualifying rows, which maps to a nil pointer and
	// lets the profile show "No ratings yet" rather than a fabricated 0.00.
	if err := r.verifiedMarketerReviews(ctx, query.MarketerID).
		Select("COUNT(*) AS total, AVG(review.rating)::float8 AS average_rating").
		Scan(&aggregate).Error; err != nil {
		return ports.MarketerReviewPage{}, err
	}
	page.Total = aggregate.Total
	page.AverageRating = aggregate.AverageRating
	if aggregate.Total == 0 {
		return page, nil
	}

	if err := r.verifiedMarketerReviews(ctx, query.MarketerID).
		Select("review.review_id, review.job_id, review.rating, review.comment, review.created_at").
		Order("review.created_at DESC, review.review_id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&page.Items).Error; err != nil {
		return ports.MarketerReviewPage{}, err
	}
	return page, nil
}
