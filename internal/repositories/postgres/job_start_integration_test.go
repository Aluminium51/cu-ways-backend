package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func jobStatusOf(t *testing.T, db *gorm.DB, jobID int32) (domain.JobStatus, *int32) {
	t.Helper()
	var job domain.Job
	if err := db.First(&job, "job_id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	return job.JobStatus, job.AcceptedOfferID
}

func TestAcceptedOfferTriggerAcceptsPendingJobAndSyncsOffers(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "trigger-sync")

	// Update only the column, as any code path might, and let the triggers
	// derive the rest.
	if err := db.Exec(`UPDATE jobs SET accepted_offer_id = ? WHERE job_id = ?`, f.hiredOffer.OfferID, f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	status, accepted := jobStatusOf(t, db, f.jobID)
	if status != domain.JobStatusAccepted || accepted == nil || *accepted != f.hiredOffer.OfferID {
		t.Fatalf("job is %s with accepted offer %v", status, accepted)
	}
	if got := offerStatus(t, db, f.hiredOffer.OfferID); got != domain.OfferStatusAccepted {
		t.Fatalf("accepted offer is %s", got)
	}
	if got := offerStatus(t, db, f.competingOffer.OfferID); got != domain.OfferStatusRejected {
		t.Fatalf("competing offer is %s", got)
	}
}

func TestAcceptedOfferTriggerRaisesMismatchAndAbortsUpdate(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "trigger-mismatch")
	other := newOfferAcceptanceFixture(t, db, "trigger-mismatch-other")
	if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.competingOffer.OfferID).Update("offer_status", domain.OfferStatusWithdrawn).Error; err != nil {
		t.Fatal(err)
	}

	for name, offerID := range map[string]int32{
		"offer_from_another_job": other.hiredOffer.OfferID,
		"withdrawn_offer":        f.competingOffer.OfferID,
	} {
		t.Run(name, func(t *testing.T) {
			// The raised exception aborts the statement; a savepoint keeps the
			// shared test transaction usable for the assertions.
			if err := db.SavePoint("before_mismatch").Error; err != nil {
				t.Fatal(err)
			}
			err := db.Exec(`UPDATE jobs SET accepted_offer_id = ? WHERE job_id = ?`, offerID, f.jobID).Error
			if rollbackErr := db.RollbackTo("before_mismatch").Error; rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != acceptedOfferMismatchCode || pgErr.ConstraintName != "trg_validate_job_accepted_offer" {
				t.Fatalf("expected the mismatch exception, got %v", err)
			}
			if !errors.Is(mapAcceptedOfferTriggerError(err), domain.ErrAcceptedOfferMismatch) {
				t.Fatalf("mismatch exception was not mapped: %v", err)
			}
		})
	}

	status, accepted := jobStatusOf(t, db, f.jobID)
	if status != domain.JobStatusPending || accepted != nil {
		t.Fatalf("aborted update changed the job: %s %v", status, accepted)
	}
	if got := offerStatus(t, db, f.hiredOffer.OfferID); got != domain.OfferStatusPending {
		t.Fatalf("aborted update changed an offer: %s", got)
	}
	if got := offerStatus(t, db, other.hiredOffer.OfferID); got != domain.OfferStatusPending {
		t.Fatalf("aborted update changed another job's offer: %s", got)
	}
}

func TestMarketerJobRepositoryStartJobMovesAcceptedJobToInProgress(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	f := newOfferAcceptanceFixture(t, db, "start-happy")
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}

	started, err := NewMarketerJobRepository(db).StartJob(ctx, f.hiredOffer.UserID, f.jobID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Job.JobStatus != domain.JobStatusInProgress || started.Offer.OfferID != f.hiredOffer.OfferID || started.Offer.OfferStatus != domain.OfferStatusAccepted {
		t.Fatalf("unexpected start result: %+v", started)
	}
	status, accepted := jobStatusOf(t, db, f.jobID)
	if status != domain.JobStatusInProgress || accepted == nil || *accepted != f.hiredOffer.OfferID {
		t.Fatalf("job was not persisted as In Progress: %s %v", status, accepted)
	}
	if got := offerStatus(t, db, f.hiredOffer.OfferID); got != domain.OfferStatusAccepted {
		t.Fatalf("starting changed the accepted offer to %s", got)
	}
}

func TestMarketerJobRepositoryStartJobRejectsInvalidStarts(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	repo := NewMarketerJobRepository(db)
	f := newOfferAcceptanceFixture(t, db, "start-guard")
	pending := newOfferAcceptanceFixture(t, db, "start-guard-pending")
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.StartJob(ctx, f.competingOffer.UserID, f.jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected a marketer without the accepted offer to be forbidden, got %v", err)
	}
	if _, err := repo.StartJob(ctx, pending.hiredOffer.UserID, pending.jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected a job without an accepted offer to be forbidden, got %v", err)
	}
	if _, err := repo.StartJob(ctx, f.hiredOffer.UserID, f.jobID+100000); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected a missing job to be not found, got %v", err)
	}
	if status, _ := jobStatusOf(t, db, f.jobID); status != domain.JobStatusAccepted {
		t.Fatalf("rejected starts changed the job to %s", status)
	}

	if _, err := repo.StartJob(ctx, f.hiredOffer.UserID, f.jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.StartJob(ctx, f.hiredOffer.UserID, f.jobID); !errors.Is(err, domain.ErrJobNotAccepted) {
		t.Fatalf("expected a started job to refuse a second start, got %v", err)
	}
}

func TestMarketerJobRepositoryStartJobRejectsAcceptedOfferThatIsNoLongerAccepted(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	f := newOfferAcceptanceFixture(t, db, "start-mismatch")
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.hiredOffer.OfferID).Update("offer_status", domain.OfferStatusWithdrawn).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewMarketerJobRepository(db).StartJob(ctx, f.hiredOffer.UserID, f.jobID); !errors.Is(err, domain.ErrAcceptedOfferMismatch) {
		t.Fatalf("expected an accepted offer mismatch, got %v", err)
	}
	if status, _ := jobStatusOf(t, db, f.jobID); status != domain.JobStatusAccepted {
		t.Fatalf("mismatched start changed the job to %s", status)
	}
}
