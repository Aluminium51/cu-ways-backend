package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

func TestReviewRepositoryCreatesOneReviewForOwnedCompletedJob(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "review-owner")
	marketerID := createIntegrationMarketer(t, db, "review-marketer")
	jobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)
	repo := NewReviewRepository(db)
	comment := "Delivered on time"
	createdAt := time.Date(2026, 10, 29, 4, 5, 6, 0, time.UTC)
	review, err := repo.CreateForCreatorJob(context.Background(), creatorID, jobID, &domain.Review{Rating: 5, Comment: &comment, CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if review.ReviewID < 1 || review.JobID != jobID || review.Rating != 5 || review.Comment == nil || *review.Comment != comment || !review.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected review: %+v", review)
	}
	var stored domain.Review
	if err := db.Where("job_id = ?", jobID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Rating != 5 || stored.Comment == nil || *stored.Comment != comment || !stored.CreatedAt.Equal(createdAt) {
		t.Fatalf("review was not stored as submitted: %+v", stored)
	}
	_, err = repo.CreateForCreatorJob(context.Background(), creatorID, jobID, &domain.Review{Rating: 4, CreatedAt: createdAt})
	if !errors.Is(err, domain.ErrReviewAlreadyExists) {
		t.Fatalf("duplicate review: expected conflict, got %v", err)
	}
}

func TestReviewRepositoryRejectsForeignMissingAndIncompleteJobs(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ownerID := createIntegrationCreator(t, db, "review-owner-errors")
	otherID := createIntegrationCreator(t, db, "review-other")
	marketerID := createIntegrationMarketer(t, db, "review-marketer-errors")
	completedID := createStatisticsJob(t, db, ownerID, marketerID, domain.JobStatusCompleted)
	pendingID := createStatisticsJob(t, db, ownerID, marketerID, domain.JobStatusPending)
	repo := NewReviewRepository(db)
	for _, tc := range []struct {
		name      string
		creatorID int32
		jobID     int32
		want      error
	}{
		{name: "foreign", creatorID: otherID, jobID: completedID, want: domain.ErrJobForbidden},
		{name: "missing", creatorID: ownerID, jobID: completedID + pendingID + 100000, want: domain.ErrJobNotFound},
		{name: "pending", creatorID: ownerID, jobID: pendingID, want: domain.ErrJobNotCompleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.CreateForCreatorJob(context.Background(), tc.creatorID, tc.jobID, &domain.Review{Rating: 3, CreatedAt: time.Now().UTC()})
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestReviewRepositoryFindsReviewForOwnedJob(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ownerID := createIntegrationCreator(t, db, "review-current")
	otherID := createIntegrationCreator(t, db, "review-current-other")
	marketerID := createIntegrationMarketer(t, db, "review-current")
	jobID := createStatisticsJob(t, db, ownerID, marketerID, domain.JobStatusCompleted)
	repo := NewReviewRepository(db)
	ctx := context.Background()

	if review, err := repo.FindForCreatorJob(ctx, ownerID, jobID); err != nil || review != nil {
		t.Fatalf("unreviewed job: %+v, %v", review, err)
	}
	comment := "Line one\nLine two"
	createdAt := time.Date(2026, 10, 29, 4, 5, 6, 0, time.UTC)
	created, err := repo.CreateForCreatorJob(ctx, ownerID, jobID, &domain.Review{Rating: 4, Comment: &comment, CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	review, err := repo.FindForCreatorJob(ctx, ownerID, jobID)
	if err != nil || review == nil || review.ReviewID != created.ReviewID || review.Rating != 4 || review.Comment == nil || *review.Comment != comment || !review.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected review %d, got %+v, %v", created.ReviewID, review, err)
	}

	if _, err := repo.FindForCreatorJob(ctx, otherID, jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("foreign creator: %v", err)
	}
	if _, err := repo.FindForCreatorJob(ctx, ownerID, jobID+100000); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("missing job: %v", err)
	}
}
