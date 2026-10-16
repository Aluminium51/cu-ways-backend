package postgres

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

type offerAcceptanceFixture struct {
	creatorID      int32
	jobID          int32
	hiredOffer     *domain.Offer
	competingOffer *domain.Offer
	hiredRequest   int32
	competingReq   int32
	pendingReq     int32
	declinedReq    int32
}

// newOfferAcceptanceFixture builds a Pending job with four invited marketers:
// one hired candidate and one competitor with Pending offers, one who has not
// answered, and one who declined.
func newOfferAcceptanceFixture(t *testing.T, db *gorm.DB, label string) offerAcceptanceFixture {
	t.Helper()
	ctx := context.Background()
	creatorID := createIntegrationCreator(t, db, label+"-creator")
	hired := createIntegrationMarketer(t, db, label+"-hired")
	competitor := createIntegrationMarketer(t, db, label+"-competitor")
	silent := createIntegrationMarketer(t, db, label+"-silent")
	decliner := createIntegrationMarketer(t, db, label+"-decliner")
	surveyID := createIntegrationSurvey(t, db, creatorID, label)
	job, err := NewJobRepository(db).CreateWithSurveys(ctx, &domain.Job{UserID: creatorID, JobStatus: domain.JobStatusPending, CreatedAt: newIntegrationOffer("0").CreatedAt}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	fixture := offerAcceptanceFixture{creatorID: creatorID, jobID: job.JobID}
	fixture.hiredOffer = submitIntegrationOffer(t, db, creatorID, job.JobID, hired, "9000")
	fixture.competingOffer = submitIntegrationOffer(t, db, creatorID, job.JobID, competitor, "8000")
	fixture.hiredRequest = *fixture.hiredOffer.JobRequestID
	fixture.competingReq = *fixture.competingOffer.JobRequestID

	requests, err := NewJobRepository(db).CreateRequests(ctx, creatorID, job.JobID, []int32{silent, decliner}, newIntegrationOffer("0").CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	fixture.pendingReq = requests[0].JobRequestID
	fixture.declinedReq = requests[1].JobRequestID
	if err := db.Model(&domain.JobRequest{}).Where("job_request_id = ?", fixture.declinedReq).Update("request_status", domain.JobRequestStatusDeclined).Error; err != nil {
		t.Fatal(err)
	}
	return fixture
}

func offerStatus(t *testing.T, db *gorm.DB, offerID int32) domain.OfferStatus {
	t.Helper()
	var offer domain.Offer
	if err := db.First(&offer, "offer_id = ?", offerID).Error; err != nil {
		t.Fatal(err)
	}
	return offer.OfferStatus
}

func requestStatus(t *testing.T, db *gorm.DB, requestID int32) domain.JobRequestStatus {
	t.Helper()
	var request domain.JobRequest
	if err := db.First(&request, "job_request_id = ?", requestID).Error; err != nil {
		t.Fatal(err)
	}
	return request.Status
}

func TestJobOfferRepositoryAcceptOfferHiresMarketerAndResolvesCompetition(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "accept-happy")

	result, err := NewJobOfferRepository(db).AcceptOffer(context.Background(), f.creatorID, f.jobID, f.hiredOffer.OfferID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.JobStatus != domain.JobStatusAccepted || result.Job.AcceptedOfferID == nil || *result.Job.AcceptedOfferID != f.hiredOffer.OfferID {
		t.Fatalf("unexpected job result: %+v", result.Job)
	}
	if !slices.Equal(result.RejectedOfferIDs, []int32{f.competingOffer.OfferID}) {
		t.Fatalf("expected the competing offer to be rejected, got %v", result.RejectedOfferIDs)
	}
	wantClosed := []int32{f.competingReq, f.pendingReq}
	slices.Sort(wantClosed)
	if !slices.Equal(result.ClosedJobRequestIDs, wantClosed) {
		t.Fatalf("expected requests %v to close, got %v", wantClosed, result.ClosedJobRequestIDs)
	}

	var job domain.Job
	if err := db.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusAccepted || job.AcceptedOfferID == nil || *job.AcceptedOfferID != f.hiredOffer.OfferID {
		t.Fatalf("job was not persisted as accepted: %+v", job)
	}
	if got := offerStatus(t, db, f.hiredOffer.OfferID); got != domain.OfferStatusAccepted {
		t.Fatalf("hired offer is %s", got)
	}
	if got := offerStatus(t, db, f.competingOffer.OfferID); got != domain.OfferStatusRejected {
		t.Fatalf("competing offer is %s", got)
	}
	for requestID, want := range map[int32]domain.JobRequestStatus{
		f.hiredRequest: domain.JobRequestStatusAccepted,
		f.competingReq: domain.JobRequestStatusClosed,
		f.pendingReq:   domain.JobRequestStatusClosed,
		f.declinedReq:  domain.JobRequestStatusDeclined,
	} {
		if got := requestStatus(t, db, requestID); got != want {
			t.Fatalf("request %d is %s, want %s", requestID, got, want)
		}
	}
}

func TestJobOfferRepositoryAcceptOfferRejectsInvalidTransitions(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	repo := NewJobOfferRepository(db)
	f := newOfferAcceptanceFixture(t, db, "accept-guard")
	other := newOfferAcceptanceFixture(t, db, "accept-guard-other")

	if _, err := repo.AcceptOffer(ctx, other.creatorID, f.jobID, f.hiredOffer.OfferID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected another creator to be forbidden, got %v", err)
	}
	if _, err := repo.AcceptOffer(ctx, f.creatorID, f.jobID+100000, f.hiredOffer.OfferID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected a missing job to be not found, got %v", err)
	}
	if _, err := repo.AcceptOffer(ctx, f.creatorID, f.jobID, other.hiredOffer.OfferID); !errors.Is(err, domain.ErrOfferNotFound) {
		t.Fatalf("expected an offer from another job to be not found, got %v", err)
	}
	if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.competingOffer.OfferID).Update("offer_status", domain.OfferStatusWithdrawn).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AcceptOffer(ctx, f.creatorID, f.jobID, f.competingOffer.OfferID); !errors.Is(err, domain.ErrOfferNotPending) {
		t.Fatalf("expected a withdrawn offer to be rejected, got %v", err)
	}

	if _, err := repo.AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); !errors.Is(err, domain.ErrJobNotPending) {
		t.Fatalf("expected an accepted job to refuse a second acceptance, got %v", err)
	}
}

func TestJobOfferRepositoryAcceptOfferRollsBackEveryChangeOnFailure(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "accept-rollback")

	// Fail the final job update, after requests are closed and offers are
	// rejected and accepted inside the same transaction.
	jobUpdateFailure := errors.New("injected job update failure")
	callbackName := "test:fail_offer_acceptance_job_update"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "jobs" {
			_ = tx.AddError(jobUpdateFailure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := NewJobOfferRepository(db).AcceptOffer(context.Background(), f.creatorID, f.jobID, f.hiredOffer.OfferID)
	if removeErr := db.Callback().Update().Remove(callbackName); removeErr != nil {
		t.Fatal(removeErr)
	}
	if !errors.Is(err, jobUpdateFailure) {
		t.Fatalf("expected the injected failure, got %v", err)
	}

	var job domain.Job
	if err := db.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil {
		t.Fatalf("job changed despite rollback: %+v", job)
	}
	for _, offerID := range []int32{f.hiredOffer.OfferID, f.competingOffer.OfferID} {
		if got := offerStatus(t, db, offerID); got != domain.OfferStatusPending {
			t.Fatalf("offer %d is %s after rollback", offerID, got)
		}
	}
	for _, requestID := range []int32{f.hiredRequest, f.competingReq} {
		if got := requestStatus(t, db, requestID); got != domain.JobRequestStatusAccepted {
			t.Fatalf("request %d is %s after rollback", requestID, got)
		}
	}
	if got := requestStatus(t, db, f.pendingReq); got != domain.JobRequestStatusPending {
		t.Fatalf("pending request is %s after rollback", got)
	}
}
