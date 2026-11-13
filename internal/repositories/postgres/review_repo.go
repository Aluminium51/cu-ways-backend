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
		var selfHire bool
		if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM offers WHERE offer_id = ? AND job_id = ? AND user_id = ?)", job.AcceptedOfferID, jobID, creatorID).Scan(&selfHire).Error; err != nil {
			return err
		}
		if selfHire {
			return domain.ErrSelfHiring
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

// FindForCreatorJob returns the owned Job's review, or nil when it has none.
// It reads only, so it works for a Job in any status.
func (r *ReviewRepository) FindForCreatorJob(ctx context.Context, creatorID, jobID int32) (*domain.Review, error) {
	db := r.db.WithContext(ctx)
	if _, err := findOwnedJob(db, creatorID, jobID); err != nil {
		return nil, err
	}
	var reviews []domain.Review
	if err := db.Where("job_id = ?", jobID).Limit(1).Find(&reviews).Error; err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return nil, nil
	}
	return &reviews[0], nil
}

// verifiedMarketerReviews selects valid reviews from Completed Jobs where the
// marketer held the accepted Offer.
func (r *ReviewRepository) verifiedMarketerReviews(ctx context.Context, marketerID int32) *gorm.DB {
	return r.db.WithContext(ctx).Table("reviews AS review").
		Joins("JOIN jobs AS job ON job.job_id = review.job_id").
		Joins("JOIN offers AS accepted_offer ON accepted_offer.job_id = job.job_id AND accepted_offer.offer_id = job.accepted_offer_id").
		Where("job.job_status = ?", domain.JobStatusCompleted).
		Where("accepted_offer.user_id = ? AND accepted_offer.user_id <> job.user_id", marketerID).
		Where("review.rating BETWEEN 1 AND 5")
}

func (r *ReviewRepository) ListForMarketer(ctx context.Context, query ports.MarketerReviewQuery) (ports.MarketerReviewPage, error) {
	page := ports.MarketerReviewPage{Page: query.Page, PageSize: query.PageSize, Items: []ports.MarketerReview{}}

	var aggregate struct {
		Total         int64
		AverageRating *float64
	}
	// AVG returns NULL when there are no qualifying reviews.
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
		Joins("JOIN users AS creator ON creator.user_id = job.user_id").
		Select("review.review_id, review.job_id, review.rating, review.comment, review.created_at, creator.name AS reviewer_name").
		Order("review.created_at DESC, review.review_id DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Scan(&page.Items).Error; err != nil {
		return ports.MarketerReviewPage{}, err
	}
	return page, nil
}
