package postgres

import (
	"context"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

func TestMarketerProfileRepositoryFindDetailReturnsActiveServicesAndVerifiedPerformance(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	marketerID := createIntegrationMarketer(t, db, "detail-target")
	otherMarketerID := createIntegrationMarketer(t, db, "detail-other")
	creatorID := createIntegrationCreator(t, db, "detail-creator")

	activeService := createIntegrationService(t, db, marketerID, "Active detail service", "500.00")
	removedService := createIntegrationService(t, db, marketerID, "Removed detail service", "900.00")
	if err := NewServiceRepository(db).Delete(context.Background(), marketerID, removedService.ServiceID); err != nil {
		t.Fatal(err)
	}

	reviewedJobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)
	createStatisticsReview(t, db, reviewedJobID, 4)
	createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)

	inProgressJobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusInProgress)
	createStatisticsReview(t, db, inProgressJobID, 5)
	otherJobID := createStatisticsJob(t, db, creatorID, otherMarketerID, domain.JobStatusCompleted)
	createStatisticsReview(t, db, otherJobID, 1)

	detail, err := NewMarketerProfileRepository(db).FindDetail(context.Background(), marketerID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Marketer.User == nil || detail.Marketer.User.UserID != marketerID {
		t.Fatalf("expected marketer user profile, got %+v", detail.Marketer.User)
	}
	if len(detail.Marketer.Services) != 1 || detail.Marketer.Services[0].ServiceID != activeService.ServiceID {
		t.Fatalf("expected only active service %d, got %+v", activeService.ServiceID, detail.Marketer.Services)
	}
	if detail.TotalCompletedJobs != 2 {
		t.Fatalf("expected 2 accepted-offer completed jobs, got %d", detail.TotalCompletedJobs)
	}
	if detail.AverageRating == nil || *detail.AverageRating != 4 {
		t.Fatalf("expected average rating 4, got %v", detail.AverageRating)
	}
}

func TestMarketerProfileRepositoryFindDetailReturnsNullRatingWithoutReviews(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	marketerID := createIntegrationMarketer(t, db, "detail-unrated")
	creatorID := createIntegrationCreator(t, db, "detail-unrated-creator")
	createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)

	detail, err := NewMarketerProfileRepository(db).FindDetail(context.Background(), marketerID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.TotalCompletedJobs != 1 || detail.AverageRating != nil {
		t.Fatalf("unexpected unrated performance: %+v", detail)
	}
}
