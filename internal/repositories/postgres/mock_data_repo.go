package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type MockDataRepository struct {
	db *gorm.DB
}

var _ ports.MockDataRepository = (*MockDataRepository)(nil)

func NewMockDataRepository(db *gorm.DB) *MockDataRepository {
	return &MockDataRepository{db: db}
}

func (r *MockDataRepository) SeedMockData(ctx context.Context, seed ports.MockDataSeed) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userIDs := make(map[string]int32, len(seed.Users))
		for _, user := range seed.Users {
			var row struct {
				UserID    int32      `gorm:"column:user_id"`
				DeletedAt *time.Time `gorm:"column:deleted_at"`
			}
			if err := tx.Raw(`
INSERT INTO users (name, email, phone, line_id, password_hash, role, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (email) DO UPDATE SET
  password_hash = COALESCE(users.password_hash, EXCLUDED.password_hash),
  phone = COALESCE(users.phone, EXCLUDED.phone),
  line_id = COALESCE(users.line_id, EXCLUDED.line_id)
RETURNING user_id, deleted_at`,
				user.Name, user.Email, user.Phone, user.LineID, user.PasswordHash, user.Role, user.CreatedAt,
			).Scan(&row).Error; err != nil {
				return err
			}
			if row.DeletedAt != nil {
				return fmt.Errorf("mock user %s is soft-deleted", user.Email)
			}
			userIDs[user.Email] = row.UserID

			if user.Creator {
				if err := tx.Exec(`INSERT INTO creators (user_id) VALUES (?) ON CONFLICT (user_id) DO NOTHING`, row.UserID).Error; err != nil {
					return err
				}
			}
			if user.Marketer != nil {
				if err := r.seedMarketer(tx, row.UserID, *user.Marketer); err != nil {
					return err
				}
			}
		}

		for _, job := range seed.Jobs {
			creatorID, ok := userIDs[job.CreatorEmail]
			if !ok {
				return fmt.Errorf("mock job references unknown creator %s", job.CreatorEmail)
			}
			marketerID, ok := userIDs[job.MarketerEmail]
			if !ok {
				return fmt.Errorf("mock job references unknown marketer %s", job.MarketerEmail)
			}
			if err := r.seedCompletedJob(tx, creatorID, marketerID, job); err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *MockDataRepository) seedMarketer(tx *gorm.DB, userID int32, seed ports.MockMarketerSeed) error {
	if err := tx.Exec(`
INSERT INTO marketers (user_id, bio, experience_years, availability_status, availability_text)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (user_id) DO UPDATE SET
  bio = EXCLUDED.bio,
  experience_years = EXCLUDED.experience_years,
  availability_status = EXCLUDED.availability_status,
  availability_text = EXCLUDED.availability_text`,
		userID, seed.Bio, seed.ExperienceYears, seed.AvailabilityStatus, seed.AvailabilityText,
	).Error; err != nil {
		return err
	}

	for _, slug := range seed.ExpertiseSlugs {
		var optionID int32
		if err := tx.Raw(`SELECT expertise_id FROM expertise_options WHERE slug = ?`, slug).Row().Scan(&optionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("expertise catalog option %q not found", slug)
			}
			return err
		}
		if err := tx.Exec(`INSERT INTO marketer_expertise (user_id, expertise_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, userID, optionID).Error; err != nil {
			return err
		}
	}
	for _, slug := range seed.CampusSlugs {
		var optionID int32
		if err := tx.Raw(`SELECT campus_id FROM campus_options WHERE slug = ?`, slug).Row().Scan(&optionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("campus catalog option %q not found", slug)
			}
			return err
		}
		if err := tx.Exec(`INSERT INTO marketer_campuses (user_id, campus_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, userID, optionID).Error; err != nil {
			return err
		}
	}

	for _, service := range seed.Services {
		var serviceID int32
		err := tx.Raw(`
SELECT service_id
FROM services
WHERE user_id = ? AND service_type = ?
ORDER BY service_id ASC
LIMIT 1`, userID, service.ServiceType).Row().Scan(&serviceID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if err := tx.Exec(`
INSERT INTO services (user_id, service_type, scope_text, price, created_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?)`, userID, service.ServiceType, service.ScopeText, service.Price, service.CreatedAt, service.DeletedAt).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if err := tx.Exec(`
UPDATE services
SET scope_text = ?, price = ?, deleted_at = ?
WHERE service_id = ?`, service.ScopeText, service.Price, service.DeletedAt, serviceID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *MockDataRepository) seedCompletedJob(tx *gorm.DB, creatorID, marketerID int32, seed ports.MockJobSeed) error {
	var jobID int32
	err := tx.Raw(`
SELECT job.job_id
FROM jobs AS job
JOIN offers AS accepted_offer
  ON accepted_offer.job_id = job.job_id
 AND accepted_offer.offer_id = job.accepted_offer_id
WHERE job.user_id = ?
  AND accepted_offer.user_id = ?
  AND job.job_status = 'Completed'
ORDER BY job.job_id ASC
LIMIT 1`, creatorID, marketerID).Row().Scan(&jobID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Raw(`
INSERT INTO jobs (user_id, job_status, created_at)
VALUES (?, 'Pending', ?)
RETURNING job_id`, creatorID, seed.CreatedAt).Row().Scan(&jobID); err != nil {
			return err
		}
		var offerID int32
		if err := tx.Raw(`
INSERT INTO offers (job_id, user_id, offered_price, offer_status, created_at)
VALUES (?, ?, ?, 'Accepted', ?)
RETURNING offer_id`, jobID, marketerID, seed.OfferedPrice, seed.CreatedAt).Row().Scan(&offerID); err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE jobs SET accepted_offer_id = ?, job_status = 'Completed' WHERE job_id = ?`, offerID, jobID).Error; err != nil {
			return err
		}
	}
	if seed.Rating != nil {
		if err := tx.Exec(`
INSERT INTO reviews (job_id, rating, created_at)
VALUES (?, ?, ?)
ON CONFLICT (job_id) DO UPDATE SET rating = EXCLUDED.rating`, jobID, *seed.Rating, seed.CreatedAt).Error; err != nil {
			return err
		}
	}
	return nil
}
