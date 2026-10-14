package postgres

import (
	"context"
	"errors"
	"slices"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type JobOfferRepository struct {
	db *gorm.DB
}

var _ ports.JobOfferRepository = (*JobOfferRepository)(nil)

func NewJobOfferRepository(db *gorm.DB) *JobOfferRepository {
	return &JobOfferRepository{db: db}
}

func (r *JobOfferRepository) ListForCreatorJob(ctx context.Context, creatorID, jobID int32, status *domain.OfferStatus) ([]ports.JobOfferComparison, error) {
	db := r.db.WithContext(ctx)
	if _, err := findOwnedJob(db, creatorID, jobID); err != nil {
		return nil, err
	}
	query := db.Preload("Marketer").Preload("Marketer.User").Where("job_id = ?", jobID)
	if status != nil {
		query = query.Where("offer_status = ?", *status)
	}
	var offers []domain.Offer
	if err := query.Order("created_at ASC, offer_id ASC").Find(&offers).Error; err != nil {
		return nil, err
	}
	if len(offers) == 0 {
		return []ports.JobOfferComparison{}, nil
	}

	marketerIDs := make([]int32, 0, len(offers))
	seen := make(map[int32]struct{}, len(offers))
	for _, offer := range offers {
		if _, exists := seen[offer.UserID]; !exists {
			seen[offer.UserID] = struct{}{}
			marketerIDs = append(marketerIDs, offer.UserID)
		}
	}
	expertise, err := marketerExpertise(db, marketerIDs)
	if err != nil {
		return nil, err
	}
	campuses, err := marketerCampuses(db, marketerIDs)
	if err != nil {
		return nil, err
	}
	ratings, err := marketerRatings(db, marketerIDs)
	if err != nil {
		return nil, err
	}

	items := make([]ports.JobOfferComparison, 0, len(offers))
	for _, offer := range offers {
		item := ports.JobOfferComparison{Offer: offer, Rating: ratings[offer.UserID]}
		if offer.Marketer != nil {
			item.Marketer = *offer.Marketer
		} else {
			item.Marketer = domain.Marketer{UserID: offer.UserID}
		}
		item.Marketer.Expertise = expertise[offer.UserID]
		item.Marketer.Campuses = campuses[offer.UserID]
		item.Offer.Marketer = nil
		items = append(items, item)
	}
	return items, nil
}

func (r *JobOfferRepository) AcceptOffer(ctx context.Context, creatorID, jobID, offerID int32) (*ports.OfferAcceptance, error) {
	var result ports.OfferAcceptance
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockOwnedJob(tx, creatorID, jobID)
		if err != nil {
			return err
		}
		if job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil {
			return domain.ErrJobNotPending
		}
		var offer domain.Offer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("offer_id = ? AND job_id = ?", offerID, jobID).
			First(&offer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrOfferNotFound
			}
			return err
		}
		if offer.OfferStatus != domain.OfferStatusPending {
			return domain.ErrOfferNotPending
		}

		// Close the other Marketers' requests first. This waits for any offer
		// submission holding one of those request locks, so an offer committed
		// concurrently is visible to the reject step below, and later
		// submissions see a Closed request.
		var closed []int32
		if err := tx.Raw(`
UPDATE job_requests SET request_status = ?
WHERE job_id = ? AND marketer_id <> ? AND request_status IN ?
RETURNING job_request_id`, domain.JobRequestStatusClosed, jobID, offer.UserID,
			[]domain.JobRequestStatus{domain.JobRequestStatusPending, domain.JobRequestStatusAccepted}).
			Scan(&closed).Error; err != nil {
			return err
		}
		var rejected []int32
		if err := tx.Raw(`
UPDATE offers SET offer_status = ?
WHERE job_id = ? AND offer_id <> ? AND offer_status = ?
RETURNING offer_id`, domain.OfferStatusRejected, jobID, offerID, domain.OfferStatusPending).
			Scan(&rejected).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Offer{}).Where("offer_id = ?", offerID).
			Update("offer_status", domain.OfferStatusAccepted).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Job{}).Where("job_id = ?", jobID).
			Updates(map[string]any{"job_status": domain.JobStatusAccepted, "accepted_offer_id": offerID}).Error; err != nil {
			return err
		}
		slices.Sort(closed)
		slices.Sort(rejected)
		if closed == nil {
			closed = []int32{}
		}
		if rejected == nil {
			rejected = []int32{}
		}

		job.JobStatus = domain.JobStatusAccepted
		job.AcceptedOfferID = &offer.OfferID
		offer.OfferStatus = domain.OfferStatusAccepted
		result = ports.OfferAcceptance{Job: *job, Offer: offer, RejectedOfferIDs: rejected, ClosedJobRequestIDs: closed}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func marketerExpertise(db *gorm.DB, marketerIDs []int32) (map[int32][]domain.Expertise, error) {
	var rows []struct {
		UserID int32
		domain.Expertise
	}
	if err := db.Table("expertise_options AS eo").
		Select("me.user_id, eo.expertise_id, eo.slug, eo.name").
		Joins("JOIN marketer_expertise AS me ON me.expertise_id = eo.expertise_id").
		Where("me.user_id IN ?", marketerIDs).
		Order("me.user_id ASC, eo.expertise_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int32][]domain.Expertise, len(marketerIDs))
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Expertise)
	}
	return result, nil
}

func marketerCampuses(db *gorm.DB, marketerIDs []int32) (map[int32][]domain.Campus, error) {
	var rows []struct {
		UserID int32
		domain.Campus
	}
	if err := db.Table("campus_options AS co").
		Select("mc.user_id, co.campus_id, co.slug, co.name").
		Joins("JOIN marketer_campuses AS mc ON mc.campus_id = co.campus_id").
		Where("mc.user_id IN ?", marketerIDs).
		Order("mc.user_id ASC, co.campus_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int32][]domain.Campus, len(marketerIDs))
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row.Campus)
	}
	return result, nil
}

// marketerRatings uses the same rule as the marketer detail endpoint: jobs
// count when Completed through the Marketer's accepted offer, and only 1-5
// reviews contribute to the average.
func marketerRatings(db *gorm.DB, marketerIDs []int32) (map[int32]ports.MarketerRating, error) {
	var rows []struct {
		MarketerID    int32
		AverageRating *float64
		ReviewCount   int64
		CompletedJobs int64
	}
	if err := db.Raw(`
SELECT
    accepted_offer.user_id AS marketer_id,
    AVG(review.rating) FILTER (WHERE review.rating BETWEEN 1 AND 5)::float8 AS average_rating,
    COUNT(review.review_id) FILTER (WHERE review.rating BETWEEN 1 AND 5) AS review_count,
    COUNT(*) AS completed_jobs
FROM jobs AS job
JOIN offers AS accepted_offer
  ON accepted_offer.job_id = job.job_id
 AND accepted_offer.offer_id = job.accepted_offer_id
LEFT JOIN reviews AS review
  ON review.job_id = job.job_id
WHERE job.job_status = ?
  AND accepted_offer.user_id IN ?
GROUP BY accepted_offer.user_id`, domain.JobStatusCompleted, marketerIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int32]ports.MarketerRating, len(rows))
	for _, row := range rows {
		result[row.MarketerID] = ports.MarketerRating{
			AverageRating: row.AverageRating,
			ReviewCount:   row.ReviewCount,
			CompletedJobs: row.CompletedJobs,
		}
	}
	return result, nil
}
