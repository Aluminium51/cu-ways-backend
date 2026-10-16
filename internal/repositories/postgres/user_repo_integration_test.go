package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CreateWithCreator opens its own transaction, which cannot nest inside the
// rolled-back transaction the other integration tests use. These tests commit
// for real and delete their own rows afterwards.
func openRegistrationIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	if err := validateIntegrationDatabaseURL(databaseURL); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newRegistrationUser(t *testing.T, db *gorm.DB) *domain.User {
	t.Helper()
	stamp := time.Now().UnixNano()
	email := fmt.Sprintf("user-repo-%d@example.invalid", stamp)
	phone := fmt.Sprintf("09%08d", stamp%100000000)
	hash := "argon2id-test-hash"
	t.Cleanup(func() {
		for _, table := range []string{"marketers", "creators"} {
			db.Exec("DELETE FROM "+table+" WHERE user_id IN (SELECT user_id FROM users WHERE email = ?)", email)
		}
		db.Exec("DELETE FROM users WHERE email = ?", email)
	})
	return &domain.User{
		Name:         "Registration Test",
		Email:        email,
		Phone:        &phone,
		PasswordHash: &hash,
		Role:         domain.RoleUser,
		CreatedAt:    time.Now().UTC(),
	}
}

func countRows(t *testing.T, db *gorm.DB, table string, userID int32) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func TestUserRepositoryCreateWithCreatorAndMarketerProfile(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)
	marketer := &domain.Marketer{
		Bio:                "Survey recruiter",
		ExperienceYears:    3,
		AvailabilityStatus: domain.AvailabilityAvailable,
		AvailabilityText:   "Weekends",
	}

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, marketer); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "creators", user.UserID) != 1 {
		t.Fatal("expected a Creator membership")
	}
	var saved domain.Marketer
	if err := db.Where("user_id = ?", user.UserID).First(&saved).Error; err != nil {
		t.Fatalf("expected a marketer row: %v", err)
	}
	if saved.Bio != "Survey recruiter" || saved.ExperienceYears != 3 || saved.AvailabilityText != "Weekends" || saved.AvailabilityStatus != domain.AvailabilityAvailable {
		t.Fatalf("unexpected marketer row: %+v", saved)
	}
}

func TestUserRepositoryCreateWithCreatorOnlyHasNoMarketerRow(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, nil); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "creators", user.UserID) != 1 || countRows(t, db, "marketers", user.UserID) != 0 {
		t.Fatal("expected a Creator membership and no marketer row")
	}
}

func TestUserRepositoryCreateWithCreatorRollsBackWhenMarketerInsertFails(t *testing.T) {
	db := openRegistrationIntegrationDB(t)
	user := newRegistrationUser(t, db)
	// Breaks the marketers.experience_years CHECK (0-80) so the last write fails.
	marketer := &domain.Marketer{ExperienceYears: 200, AvailabilityStatus: domain.AvailabilityAvailable}

	if err := NewUserRepository(db).CreateWithCreator(context.Background(), user, marketer); err == nil {
		t.Fatal("expected the marketer insert to fail")
	}
	var users int64
	if err := db.Table("users").Where("email = ?", user.Email).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Fatal("a failed marketer insert must not leave the user behind")
	}
}
