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

// execGuarded runs one statement inside a savepoint, so a guard exception does
// not abort the shared test transaction, and returns the statement's error.
func execGuarded(t *testing.T, db *gorm.DB, sql string, args ...any) error {
	t.Helper()
	if err := db.SavePoint("before_guard").Error; err != nil {
		t.Fatal(err)
	}
	err := db.Exec(sql, args...).Error
	if rollbackErr := db.RollbackTo("before_guard").Error; rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	return err
}

func requireGuardViolation(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "trg_guard_job_accepted_offer" {
		t.Fatalf("expected the accepted offer guard to refuse the update, got %v", err)
	}
}

func TestAcceptedOfferGuardRefusesInconsistentHires(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "guard-hire")
	other := newOfferAcceptanceFixture(t, db, "guard-hire-other")

	for name, update := range map[string]func() error{
		// The guard never derives statuses, so a column-only write that would
		// leave the job Pending and the offers untouched is refused.
		"job_left_pending": func() error {
			return execGuarded(t, db, `UPDATE jobs SET accepted_offer_id = ? WHERE job_id = ?`, f.hiredOffer.OfferID, f.jobID)
		},
		"offer_not_accepted": func() error {
			return execGuarded(t, db, `UPDATE jobs SET accepted_offer_id = ?, job_status = 'Accepted' WHERE job_id = ?`, f.hiredOffer.OfferID, f.jobID)
		},
		"offer_from_another_job": func() error {
			return execGuarded(t, db, `UPDATE jobs SET accepted_offer_id = ?, job_status = 'Accepted' WHERE job_id = ?`, other.hiredOffer.OfferID, f.jobID)
		},
		"competing_offer_still_pending": func() error {
			if err := db.SavePoint("competing").Error; err != nil {
				t.Fatal(err)
			}
			defer db.RollbackTo("competing")
			if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.hiredOffer.OfferID).Update("offer_status", domain.OfferStatusAccepted).Error; err != nil {
				t.Fatal(err)
			}
			return execGuarded(t, db, `UPDATE jobs SET accepted_offer_id = ?, job_status = 'Accepted' WHERE job_id = ?`, f.hiredOffer.OfferID, f.jobID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			requireGuardViolation(t, update())
		})
	}

	status, accepted := jobStatusOf(t, db, f.jobID)
	if status != domain.JobStatusPending || accepted != nil {
		t.Fatalf("refused updates changed the job: %s %v", status, accepted)
	}
	if got := offerStatus(t, db, f.hiredOffer.OfferID); got != domain.OfferStatusPending {
		t.Fatalf("refused updates changed an offer: %s", got)
	}
}

func TestAcceptedOfferGuardRefusesRehiringAfterPending(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	f := newOfferAcceptanceFixture(t, db, "guard-rehire")
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	// Reopen the competitor so pointing the job at it would otherwise look like
	// a valid hire.
	if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.competingOffer.OfferID).Update("offer_status", domain.OfferStatusPending).Error; err != nil {
		t.Fatal(err)
	}

	requireGuardViolation(t, execGuarded(t, db, `UPDATE jobs SET accepted_offer_id = ? WHERE job_id = ?`, f.competingOffer.OfferID, f.jobID))
	if _, err := NewMarketerJobRepository(db).StartJob(ctx, f.hiredOffer.UserID, f.jobID); err != nil {
		t.Fatalf("the hired marketer should still be able to start the job: %v", err)
	}
	status, accepted := jobStatusOf(t, db, f.jobID)
	if status != domain.JobStatusInProgress || accepted == nil || *accepted != f.hiredOffer.OfferID {
		t.Fatalf("job changed unexpectedly: %s %v", status, accepted)
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
