package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

// insertReview writes a review row directly so the test controls the comment and
// created_at, which drives the verified filter, average, and newest-first order.
func insertReview(t *testing.T, db *gorm.DB, jobID, rating int32, comment *string, createdAt time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO reviews (job_id, rating, comment, created_at) VALUES (?, ?, ?, ?)`, jobID, rating, comment, createdAt).Error; err != nil {
		t.Fatalf("insert review fixture: %v", err)
	}
}

func TestReviewRepositoryListsOnlyVerifiedMarketerReviews(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "verified-creator")
	otherCreatorID := createIntegrationCreator(t, db, "verified-other-creator")
	otherCreatorName := "Mali Arun"
	if err := db.Exec(`UPDATE users SET name = ? WHERE user_id = ?`, otherCreatorName, otherCreatorID).Error; err != nil {
		t.Fatalf("set reviewer name: %v", err)
	}
	marketerID := createIntegrationMarketer(t, db, "verified-marketer")
	otherMarketerID := createIntegrationMarketer(t, db, "verified-other")

	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	newer := base.Add(48 * time.Hour)
	comment := "Clear communication"

	// Two verified reviews for the marketer: Completed Jobs where the marketer
	// held the accepted Offer. The newer one must come first.
	olderJob := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)
	insertReview(t, db, olderJob, 5, &comment, base)
	newerJob := createStatisticsJob(t, db, otherCreatorID, marketerID, domain.JobStatusCompleted)
	insertReview(t, db, newerJob, 3, nil, newer)

	// Not verified: an In Progress job (not Completed) and a review for another
	// marketer. Neither should appear for this marketer, nor affect the average.
	inProgressJob := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusInProgress)
	insertReview(t, db, inProgressJob, 1, nil, newer)
	otherJob := createStatisticsJob(t, db, creatorID, otherMarketerID, domain.JobStatusCompleted)
	insertReview(t, db, otherJob, 1, nil, newer)
	// A Completed job with no review at all should not create a phantom row.
	createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)

	repo := NewReviewRepository(db)
	page, err := repo.ListForMarketer(context.Background(), ports.MarketerReviewQuery{MarketerID: marketerID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("expected 2 verified reviews, got %d", page.Total)
	}
	if page.AverageRating == nil || *page.AverageRating != 4.0 {
		t.Fatalf("expected average 4.0 over the two verified reviews, got %v", page.AverageRating)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if page.Items[0].JobID != newerJob || page.Items[1].JobID != olderJob {
		t.Fatalf("expected newest-first order, got %d then %d", page.Items[0].JobID, page.Items[1].JobID)
	}
	if page.Items[0].ReviewerName != otherCreatorName || page.Items[1].ReviewerName != "Statistics Creator" {
		t.Fatalf("expected names from the respective job owners, got %q and %q", page.Items[0].ReviewerName, page.Items[1].ReviewerName)
	}
	if page.Items[1].Rating != 5 || page.Items[1].Comment == nil || *page.Items[1].Comment != comment {
		t.Fatalf("expected rating and comment preserved, got %+v", page.Items[1])
	}
	if page.Items[0].Comment != nil {
		t.Fatalf("expected a null comment to stay null, got %v", *page.Items[0].Comment)
	}
}

func TestReviewRepositoryMarketerReviewsEmptyState(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "empty-creator")
	marketerID := createIntegrationMarketer(t, db, "empty-marketer")
	// A Completed job with the marketer hired, but no review submitted yet.
	createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)

	repo := NewReviewRepository(db)
	page, err := repo.ListForMarketer(context.Background(), ports.MarketerReviewQuery{MarketerID: marketerID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || page.AverageRating != nil || len(page.Items) != 0 {
		t.Fatalf("expected empty state with nil average, got total=%d avg=%v items=%d", page.Total, page.AverageRating, len(page.Items))
	}
}

func TestReviewRepositoryMarketerReviewsPaginate(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creatorID := createIntegrationCreator(t, db, "page-creator")
	marketerID := createIntegrationMarketer(t, db, "page-marketer")

	base := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		jobID := createStatisticsJob(t, db, creatorID, marketerID, domain.JobStatusCompleted)
		insertReview(t, db, jobID, int32(3+i), nil, base.Add(time.Duration(i)*time.Hour))
	}

	repo := NewReviewRepository(db)
	first, err := repo.ListForMarketer(context.Background(), ports.MarketerReviewQuery{MarketerID: marketerID, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Items) != 2 {
		t.Fatalf("expected total 3 with 2 on page 1, got total=%d items=%d", first.Total, len(first.Items))
	}
	second, err := repo.ListForMarketer(context.Background(), ports.MarketerReviewQuery{MarketerID: marketerID, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 3 || len(second.Items) != 1 {
		t.Fatalf("expected total 3 with 1 on page 2, got total=%d items=%d", second.Total, len(second.Items))
	}
	if first.AverageRating == nil || *first.AverageRating != 4.0 {
		t.Fatalf("expected average 4.0 across all pages, got %v", first.AverageRating)
	}
}
