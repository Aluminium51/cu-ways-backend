package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

// submitIntegrationOffer invites marketerID to the job, accepts the request,
// and submits an offer at price, returning the offer.
func submitIntegrationOffer(t *testing.T, db *gorm.DB, creatorID, jobID, marketerID int32, price string) *domain.Offer {
	t.Helper()
	requests, err := NewJobRepository(db).CreateRequests(context.Background(), creatorID, jobID, []int32{marketerID}, newIntegrationOffer("0").CreatedAt)
	if err != nil {
		t.Fatalf("create job request fixture: %v", err)
	}
	acceptIntegrationJobRequest(t, db, marketerID, requests[0].JobRequestID)
	offer := newIntegrationOffer(price)
	if err := NewMarketerJobRequestRepository(db).CreateOffer(context.Background(), marketerID, requests[0].JobRequestID, offer); err != nil {
		t.Fatalf("create offer fixture: %v", err)
	}
	return offer
}

func TestJobOfferRepositoryListsOffersWithProfilesAndRatings(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "compare-creator")
	ratedMarketer := createIntegrationMarketerNamed(t, db, "Rated Marketer", "compare-rated")
	newMarketer := createIntegrationMarketerNamed(t, db, "New Marketer", "compare-new")

	// The rated marketer completed two reviewed jobs (4 and 5 stars) and one
	// completed job without a review.
	for _, rating := range []int32{4, 5} {
		createStatisticsReview(t, db, createStatisticsJob(t, db, creatorID, ratedMarketer, domain.JobStatusCompleted), rating)
	}
	createStatisticsJob(t, db, creatorID, ratedMarketer, domain.JobStatusCompleted)

	var expertiseID int32
	if err := db.Raw(`SELECT expertise_id FROM expertise_options ORDER BY expertise_id LIMIT 1`).Scan(&expertiseID).Error; err != nil || expertiseID == 0 {
		t.Fatalf("load expertise option: id=%d err=%v", expertiseID, err)
	}
	if err := db.Exec(`INSERT INTO marketer_expertise (user_id, expertise_id) VALUES (?, ?)`, ratedMarketer, expertiseID).Error; err != nil {
		t.Fatal(err)
	}

	surveyID := createIntegrationSurvey(t, db, creatorID, "compare")
	job, err := NewJobRepository(db).CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: newIntegrationOffer("0").CreatedAt}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	ratedOffer := submitIntegrationOffer(t, db, creatorID, job.JobID, ratedMarketer, "9000")
	newOffer := submitIntegrationOffer(t, db, creatorID, job.JobID, newMarketer, "7500.50")

	items, err := NewJobOfferRepository(db).ListForCreatorJob(ctx, creatorID, job.JobID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Offer.OfferID != ratedOffer.OfferID || items[1].Offer.OfferID != newOffer.OfferID {
		t.Fatalf("expected both offers oldest first, got %+v", items)
	}

	rated := items[0]
	if rated.Marketer.User == nil || rated.Marketer.User.Name != "Rated Marketer" || rated.Marketer.Bio == "" {
		t.Fatalf("expected the marketer profile, got %+v", rated.Marketer)
	}
	if len(rated.Marketer.Expertise) != 1 || rated.Marketer.Expertise[0].ExpertiseID != expertiseID {
		t.Fatalf("expected the marketer's expertise, got %+v", rated.Marketer.Expertise)
	}
	if rated.Rating.AverageRating == nil || *rated.Rating.AverageRating != 4.5 || rated.Rating.ReviewCount != 2 || rated.Rating.CompletedJobs != 3 {
		t.Fatalf("unexpected rating: avg=%v reviews=%d jobs=%d", rated.Rating.AverageRating, rated.Rating.ReviewCount, rated.Rating.CompletedJobs)
	}

	unrated := items[1]
	if unrated.Rating.AverageRating != nil || unrated.Rating.ReviewCount != 0 || unrated.Rating.CompletedJobs != 0 {
		t.Fatalf("a marketer without history must be unrated, got %+v", unrated.Rating)
	}
	if unrated.Offer.OfferedPrice.StringFixed(2) != "7500.50" || len(unrated.Marketer.Expertise) != 0 {
		t.Fatalf("unexpected unrated offer: %+v", unrated)
	}
}

func TestJobOfferRepositoryFiltersByStatusAndChecksOwnership(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, "compare-owner")
	otherCreatorID := createIntegrationCreator(t, db, "compare-other")
	marketerID := createIntegrationMarketer(t, db, "compare-filter")
	surveyID := createIntegrationSurvey(t, db, creatorID, "compare-filter")
	job, err := NewJobRepository(db).CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: newIntegrationOffer("0").CreatedAt}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	offer := submitIntegrationOffer(t, db, creatorID, job.JobID, marketerID, "100")
	repo := NewJobOfferRepository(db)

	withdrawn := domain.OfferStatusWithdrawn
	filtered, err := repo.ListForCreatorJob(ctx, creatorID, job.JobID, &withdrawn)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("status filter returned %d non-matching offers", len(filtered))
	}
	pending := domain.OfferStatusPending
	filtered, err = repo.ListForCreatorJob(ctx, creatorID, job.JobID, &pending)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Offer.OfferID != offer.OfferID {
		t.Fatalf("expected the pending offer, got %+v", filtered)
	}

	if _, err := repo.ListForCreatorJob(ctx, otherCreatorID, job.JobID, nil); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected another creator to be forbidden, got %v", err)
	}
	if _, err := repo.ListForCreatorJob(ctx, creatorID, job.JobID+100000, nil); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected a missing job to be not found, got %v", err)
	}
	empty, err := NewJobRepository(db).CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: newIntegrationOffer("0").CreatedAt}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	none, err := repo.ListForCreatorJob(ctx, creatorID, empty.JobID, nil)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("a job without offers must return an empty list, got %v %v", none, err)
	}
}
