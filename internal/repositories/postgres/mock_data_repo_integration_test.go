package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/services"
)

func TestMockDataRepositoryIsIdempotent(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	seed, summary := services.BuildMockDataSeed(time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), "mock-password-hash")
	repo := NewMockDataRepository(db)

	if err := repo.SeedMockData(context.Background(), seed); err != nil {
		t.Fatal(err)
	}
	if err := repo.SeedMockData(context.Background(), seed); err != nil {
		t.Fatal(err)
	}

	var users, creators, marketers, services, jobs, reviews int64
	if err := db.Table("users").Where("email LIKE ?", "mock.%@example.test").Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("creators AS creator").Joins("JOIN users ON users.user_id = creator.user_id").Where("users.email LIKE ?", "mock.%@example.test").Count(&creators).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("marketers AS marketer").Joins("JOIN users ON users.user_id = marketer.user_id").Where("users.email LIKE ?", "mock.%@example.test").Count(&marketers).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("services AS service").Joins("JOIN users ON users.user_id = service.user_id").Where("users.email LIKE ? AND service.service_type LIKE 'Mock %'", "mock.%@example.test").Count(&services).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("jobs AS job").Joins("JOIN users ON users.user_id = job.user_id").Where("users.email LIKE ?", "mock.creator-only.%@example.test").Count(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("reviews AS review").Joins("JOIN jobs AS job ON job.job_id = review.job_id").Joins("JOIN users ON users.user_id = job.user_id").Where("users.email LIKE ?", "mock.creator-only.%@example.test").Count(&reviews).Error; err != nil {
		t.Fatal(err)
	}

	if users != int64(summary.Users) || creators != int64(summary.Creators) || marketers != int64(summary.Marketers) || services != 17 || jobs != int64(len(seed.Jobs)) || reviews != int64(len(seed.Jobs)) {
		t.Fatalf("unexpected mock data counts: users=%d creators=%d marketers=%d services=%d jobs=%d reviews=%d", users, creators, marketers, services, jobs, reviews)
	}
}
