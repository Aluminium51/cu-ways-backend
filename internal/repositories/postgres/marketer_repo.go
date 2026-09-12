package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type MarketerProfileRepository struct {
	db *gorm.DB
}

var _ ports.MarketerProfileRepository = (*MarketerProfileRepository)(nil)

func NewMarketerProfileRepository(db *gorm.DB) *MarketerProfileRepository {
	return &MarketerProfileRepository{db: db}
}

func (r *MarketerProfileRepository) FindProfile(ctx context.Context, userID int32) (*domain.Marketer, error) {
	var profile domain.Marketer
	if err := r.db.WithContext(ctx).
		Preload("User", "deleted_at IS NULL").
		Preload("Services", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("service_id ASC")
		}).
		Where("user_id = ? AND EXISTS (SELECT 1 FROM users WHERE users.user_id = marketers.user_id AND users.deleted_at IS NULL)", userID).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrMarketerProfileNotFound
		}
		return nil, err
	}

	if err := r.db.WithContext(ctx).
		Table("expertise_options AS eo").
		Select("eo.expertise_id, eo.slug, eo.name").
		Joins("JOIN marketer_expertise AS me ON me.expertise_id = eo.expertise_id").
		Where("me.user_id = ?", userID).
		Order("eo.expertise_id ASC").
		Find(&profile.Expertise).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Table("campus_options AS co").
		Select("co.campus_id, co.slug, co.name").
		Joins("JOIN marketer_campuses AS mc ON mc.campus_id = co.campus_id").
		Where("mc.user_id = ?", userID).
		Order("co.campus_id ASC").
		Find(&profile.Campuses).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *MarketerProfileRepository) FindDetail(ctx context.Context, userID int32) (*domain.MarketerDetail, error) {
	profile, err := r.FindProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	var performance struct {
		TotalCompletedJobs int64
		AverageRating      *float64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT
    COUNT(*) AS total_completed_jobs,
    AVG(review.rating) FILTER (WHERE review.rating BETWEEN 1 AND 5)::float8 AS average_rating
FROM jobs AS job
JOIN offers AS accepted_offer
  ON accepted_offer.job_id = job.job_id
 AND accepted_offer.offer_id = job.accepted_offer_id
LEFT JOIN reviews AS review
  ON review.job_id = job.job_id
WHERE job.job_status = ?
  AND accepted_offer.user_id = ?`, domain.JobStatusCompleted, userID).Scan(&performance).Error; err != nil {
		return nil, err
	}

	return &domain.MarketerDetail{
		Marketer:           *profile,
		TotalCompletedJobs: performance.TotalCompletedJobs,
		AverageRating:      performance.AverageRating,
	}, nil
}

func (r *MarketerProfileRepository) SaveProfile(ctx context.Context, profile *domain.Marketer, expertiseIDs, campusIDs []int32) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	rollback := func(err error) error {
		tx.Rollback()
		return err
	}

	if err := tx.Exec(`
INSERT INTO marketers (user_id, bio, experience_years, availability_status, availability_text)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET
  bio = EXCLUDED.bio,
  experience_years = EXCLUDED.experience_years,
  availability_status = EXCLUDED.availability_status,
  availability_text = EXCLUDED.availability_text`,
		profile.UserID, profile.Bio, profile.ExperienceYears, profile.AvailabilityStatus, profile.AvailabilityText).Error; err != nil {
		return rollback(err)
	}
	if err := tx.Exec("DELETE FROM marketer_expertise WHERE user_id = ?", profile.UserID).Error; err != nil {
		return rollback(err)
	}
	for _, expertiseID := range expertiseIDs {
		if err := tx.Exec("INSERT INTO marketer_expertise (user_id, expertise_id) VALUES (?, ?)", profile.UserID, expertiseID).Error; err != nil {
			return rollback(err)
		}
	}
	if err := tx.Exec("DELETE FROM marketer_campuses WHERE user_id = ?", profile.UserID).Error; err != nil {
		return rollback(err)
	}
	for _, campusID := range campusIDs {
		if err := tx.Exec("INSERT INTO marketer_campuses (user_id, campus_id) VALUES (?, ?)", profile.UserID, campusID).Error; err != nil {
			return rollback(err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}

func (r *MarketerProfileRepository) Search(ctx context.Context, query ports.MarketerSearchQuery) (ports.MarketerPage, error) {
	servicePrices := r.db.WithContext(ctx).
		Table("services").
		Select("user_id, MIN(price) AS lowest_price").
		Where("deleted_at IS NULL")
	if query.MinPrice != nil {
		servicePrices = servicePrices.Where("price >= ?", *query.MinPrice)
	}
	if query.MaxPrice != nil {
		servicePrices = servicePrices.Where("price <= ?", *query.MaxPrice)
	}
	servicePrices = servicePrices.Group("user_id")

	ratings := r.db.WithContext(ctx).Table("offers AS o").
		Select("o.user_id AS marketer_id, AVG(r.rating)::float8 AS average_rating, COUNT(r.review_id) AS review_count").
		Joins("JOIN jobs AS j ON j.job_id = o.job_id AND j.accepted_offer_id = o.offer_id AND j.job_status = ?", domain.JobStatusCompleted).
		Joins("JOIN reviews AS r ON r.job_id = j.job_id AND r.rating BETWEEN 1 AND 5").
		Group("o.user_id")

	base := r.db.WithContext(ctx).Table("marketers AS m").
		Joins("JOIN users AS u ON u.user_id = m.user_id AND u.deleted_at IS NULL").
		Joins("LEFT JOIN (?) AS sp ON sp.user_id = m.user_id", servicePrices).
		Joins("LEFT JOIN (?) AS rt ON rt.marketer_id = m.user_id", ratings)
	if query.MinPrice != nil || query.MaxPrice != nil {
		base = base.Where("sp.user_id IS NOT NULL")
	}
	if query.MinExperienceYears != nil {
		base = base.Where("m.experience_years >= ?", *query.MinExperienceYears)
	}
	if query.AvailabilityStatus != nil {
		base = base.Where("m.availability_status = ?", *query.AvailabilityStatus)
	}
	if query.MinRating != nil {
		base = base.Where("rt.average_rating IS NOT NULL AND rt.average_rating >= ?", *query.MinRating)
	}
	if len(query.ExpertiseSlugs) > 0 {
		base = base.Where(`EXISTS (
SELECT 1
FROM marketer_expertise AS me_filter
JOIN expertise_options AS eo_filter ON eo_filter.expertise_id = me_filter.expertise_id
WHERE me_filter.user_id = m.user_id AND eo_filter.slug IN ?
GROUP BY me_filter.user_id
HAVING COUNT(DISTINCT eo_filter.slug) = ?
)`, query.ExpertiseSlugs, len(query.ExpertiseSlugs))
	}
	if len(query.CampusSlugs) > 0 {
		base = base.Where(`EXISTS (
SELECT 1
FROM marketer_campuses AS mc_filter
JOIN campus_options AS co_filter ON co_filter.campus_id = mc_filter.campus_id
WHERE mc_filter.user_id = m.user_id AND co_filter.slug IN ?
GROUP BY mc_filter.user_id
HAVING COUNT(DISTINCT co_filter.slug) = ?
)`, query.CampusSlugs, len(query.CampusSlugs))
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return ports.MarketerPage{}, err
	}

	selectQuery := base.Select(`
m.user_id,
u.name,
u.email,
u.phone,
u.line_id,
u.created_at,
m.bio,
m.experience_years,
m.availability_status,
m.availability_text,
sp.lowest_price,
rt.average_rating,
COALESCE(rt.review_count, 0) AS review_count`)
	switch query.Sort {
	case "rating_desc":
		selectQuery = selectQuery.Order("rt.average_rating IS NULL ASC, rt.average_rating DESC, sp.lowest_price IS NULL ASC, sp.lowest_price ASC, m.user_id ASC")
	default:
		selectQuery = selectQuery.Order("sp.lowest_price IS NULL ASC, sp.lowest_price ASC, rt.average_rating IS NULL ASC, rt.average_rating DESC, m.user_id ASC")
	}
	type row struct {
		UserID             int32
		Name               string
		Email              string
		Phone              *string
		LineID             *string
		CreatedAt          time.Time
		Bio                string
		ExperienceYears    int32
		AvailabilityStatus domain.AvailabilityStatus
		AvailabilityText   string
		LowestPrice        *string
		AverageRating      *float64
		ReviewCount        int64
	}
	rows := make([]row, 0)
	if err := selectQuery.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return ports.MarketerPage{}, err
	}

	items := make([]domain.MarketerSearchResult, 0, len(rows))
	for _, item := range rows {
		profile, err := r.FindProfile(ctx, item.UserID)
		if err != nil {
			return ports.MarketerPage{}, err
		}
		profile.User = &domain.User{
			UserID:    item.UserID,
			Name:      item.Name,
			Email:     item.Email,
			Phone:     item.Phone,
			LineID:    item.LineID,
			CreatedAt: item.CreatedAt,
		}
		profile.Bio = item.Bio
		profile.ExperienceYears = item.ExperienceYears
		profile.AvailabilityStatus = item.AvailabilityStatus
		profile.AvailabilityText = item.AvailabilityText
		items = append(items, domain.MarketerSearchResult{
			Marketer:      *profile,
			LowestPrice:   item.LowestPrice,
			AverageRating: item.AverageRating,
			ReviewCount:   item.ReviewCount,
		})
	}
	return ports.MarketerPage{Items: items, Page: query.Page, PageSize: query.PageSize, Total: total}, nil
}
