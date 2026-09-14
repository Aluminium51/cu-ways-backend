package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestMarketerStatisticsRepositoryCalculatesCompletedJobsRatingsAndPaidEarnings(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	targetID := createIntegrationMarketer(t, db, "statistics-target")
	otherID := createIntegrationMarketer(t, db, "statistics-other")
	creatorID := createIntegrationCreator(t, db, "statistics-creator")

	reviewedJobID := createStatisticsJob(t, db, creatorID, targetID, domain.JobStatusCompleted)
	createStatisticsReview(t, db, reviewedJobID, 4)
	createStatisticsPayment(t, db, reviewedJobID, "100.00", domain.PaymentStatusPaid)
	createStatisticsPayment(t, db, reviewedJobID, "50.00", domain.PaymentStatusPaid)
	createStatisticsPayment(t, db, reviewedJobID, "30.00", domain.PaymentStatusFailed)
	createStatisticsPayment(t, db, reviewedJobID, "40.00", domain.PaymentStatusPending)

	unreviewedJobID := createStatisticsJob(t, db, creatorID, targetID, domain.JobStatusCompleted)
	createStatisticsPayment(t, db, unreviewedJobID, "75.00", domain.PaymentStatusPaid)

	inProgressJobID := createStatisticsJob(t, db, creatorID, targetID, domain.JobStatusInProgress)
	createStatisticsReview(t, db, inProgressJobID, 5)
	createStatisticsPayment(t, db, inProgressJobID, "999.00", domain.PaymentStatusPaid)
	cancelledJobID := createStatisticsJob(t, db, creatorID, targetID, domain.JobStatusCancelled)
	createStatisticsReview(t, db, cancelledJobID, 5)
	createStatisticsPayment(t, db, cancelledJobID, "999.00", domain.PaymentStatusPaid)

	otherJobID := createStatisticsJob(t, db, creatorID, otherID, domain.JobStatusCompleted)
	createStatisticsReview(t, db, otherJobID, 1)
	createStatisticsPayment(t, db, otherJobID, "500.00", domain.PaymentStatusPaid)

	result, err := NewMarketerStatisticsRepository(db).GetByMarketer(context.Background(), targetID)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCompletedJobs != 2 {
		t.Fatalf("expected 2 completed jobs, got %d", result.TotalCompletedJobs)
	}
	if result.AverageRating == nil || *result.AverageRating != 4 {
		t.Fatalf("expected average rating 4, got %v", result.AverageRating)
	}
	if !result.TotalEarnings.Equal(decimal.RequireFromString("225.00")) {
		t.Fatalf("expected paid earnings 225.00, got %s", result.TotalEarnings)
	}
}

func TestMarketerStatisticsRepositoryReturnsZeroAndNullWithoutCompletedJobs(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	marketerID := createIntegrationMarketer(t, db, "statistics-empty")

	result, err := NewMarketerStatisticsRepository(db).GetByMarketer(context.Background(), marketerID)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCompletedJobs != 0 || result.AverageRating != nil || !result.TotalEarnings.IsZero() {
		t.Fatalf("unexpected empty statistics: %+v", result)
	}
}

func createIntegrationCreator(t *testing.T, db *gorm.DB, label string) int32 {
	t.Helper()
	var userID int32
	email := fmt.Sprintf("statistics-%s-%d@example.invalid", label, time.Now().UnixNano())
	if err := db.Raw(`
INSERT INTO users (name, email, created_at)
VALUES (?, ?, ?)
RETURNING user_id`, "Statistics Creator", email, time.Now().UTC()).Scan(&userID).Error; err != nil {
		t.Fatalf("create creator user fixture: %v", err)
	}
	if err := db.Exec(`INSERT INTO creators (user_id) VALUES (?)`, userID).Error; err != nil {
		t.Fatalf("create creator fixture: %v", err)
	}
	return userID
}

func createStatisticsJob(t *testing.T, db *gorm.DB, creatorID, marketerID int32, status domain.JobStatus) int32 {
	t.Helper()
	var jobID int32
	if err := db.Raw(`
INSERT INTO jobs (user_id, job_status, created_at)
VALUES (?, ?, ?)
RETURNING job_id`, creatorID, domain.JobStatusPending, time.Now().UTC()).Scan(&jobID).Error; err != nil {
		t.Fatalf("create job fixture: %v", err)
	}
	var offerID int32
	if err := db.Raw(`
INSERT INTO offers (job_id, user_id, offered_price, offer_status, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING offer_id`, jobID, marketerID, decimal.RequireFromString("100.00"), domain.OfferStatusAccepted, time.Now().UTC()).Scan(&offerID).Error; err != nil {
		t.Fatalf("create accepted offer fixture: %v", err)
	}
	if err := db.Exec(`UPDATE jobs SET accepted_offer_id = ?, job_status = ? WHERE job_id = ?`, offerID, status, jobID).Error; err != nil {
		t.Fatalf("accept offer fixture: %v", err)
	}
	return jobID
}

func createStatisticsPayment(t *testing.T, db *gorm.DB, jobID int32, amount string, status domain.PaymentStatus) {
	t.Helper()
	if err := db.Exec(`
INSERT INTO payments (job_id, amount, method, payment_status, paid_at)
VALUES (?, ?, ?, ?, ?)`, jobID, decimal.RequireFromString(amount), domain.PaymentMethodPromptPay, status, time.Now().UTC()).Error; err != nil {
		t.Fatalf("create payment fixture: %v", err)
	}
}

func createStatisticsReview(t *testing.T, db *gorm.DB, jobID int32, rating int32) {
	t.Helper()
	if err := db.Exec(`
INSERT INTO reviews (job_id, rating, created_at)
VALUES (?, ?, ?)`, jobID, rating, time.Now().UTC()).Error; err != nil {
		t.Fatalf("create review fixture: %v", err)
	}
}
