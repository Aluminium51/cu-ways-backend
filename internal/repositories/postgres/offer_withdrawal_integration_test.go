package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"gorm.io/gorm"
)

func TestOfferWithdrawalPreservesOfferHistoryAndInvitation(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "withdraw-history")
	repo := NewMarketerJobRequestRepository(db)
	var original domain.Offer
	if err := db.First(&original, "offer_id = ?", f.hiredOffer.OfferID).Error; err != nil {
		t.Fatal(err)
	}
	withdrawn, err := repo.WithdrawOffer(context.Background(), original.UserID, original.OfferID)
	if err != nil {
		t.Fatal(err)
	}
	original.OfferStatus = domain.OfferStatusWithdrawn
	var stored domain.Offer
	if err := db.First(&stored, "offer_id = ?", original.OfferID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, stored) || !reflect.DeepEqual(original, *withdrawn) {
		t.Fatalf("only the offer status may change: original=%+v stored=%+v returned=%+v", original, stored, withdrawn)
	}
	if requestStatus(t, db, f.hiredRequest) != domain.JobRequestStatusAccepted {
		t.Fatal("withdrawal must keep the invitation Accepted for resubmission")
	}
	var job domain.Job
	if err := db.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil || offerStatus(t, db, f.competingOffer.OfferID) != domain.OfferStatusPending {
		t.Fatal("withdrawal must not change the job or competing offers")
	}
	if _, err := NewJobOfferRepository(db).AcceptOffer(context.Background(), f.creatorID, f.jobID, original.OfferID); !errors.Is(err, domain.ErrOfferNotPending) {
		t.Fatalf("a withdrawn offer cannot be accepted, got %v", err)
	}
}

func TestOfferWithdrawalRejectsForeignMissingAndResolvedOffers(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "withdraw-denied")
	repo := NewMarketerJobRequestRepository(db)
	ctx := context.Background()
	for _, id := range []int32{0, f.hiredOffer.OfferID} {
		if _, err := repo.WithdrawOffer(ctx, f.competingOffer.UserID, id); !errors.Is(err, domain.ErrOfferNotFound) {
			t.Fatalf("missing and foreign offers must be hidden, got %v", err)
		}
	}
	if offerStatus(t, db, f.hiredOffer.OfferID) != domain.OfferStatusPending {
		t.Fatal("foreign withdrawal changed the offer")
	}
	for _, status := range []domain.OfferStatus{domain.OfferStatusAccepted, domain.OfferStatusRejected, domain.OfferStatusWithdrawn} {
		if err := db.Model(&domain.Offer{}).Where("offer_id = ?", f.hiredOffer.OfferID).Update("offer_status", status).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := repo.WithdrawOffer(ctx, f.hiredOffer.UserID, f.hiredOffer.OfferID); !errors.Is(err, domain.ErrOfferNotPending) {
			t.Fatalf("%s: expected ErrOfferNotPending, got %v", status, err)
		}
		if _, err := repo.WithdrawOffer(ctx, f.competingOffer.UserID, f.hiredOffer.OfferID); !errors.Is(err, domain.ErrOfferNotFound) {
			t.Fatalf("%s: a foreign resolved offer must remain hidden, got %v", status, err)
		}
		if offerStatus(t, db, f.hiredOffer.OfferID) != status {
			t.Fatalf("%s: refused withdrawal changed the offer", status)
		}
	}
}

func TestOfferWithdrawalHonorsCancellation(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "withdraw-canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewMarketerJobRequestRepository(db).WithdrawOffer(ctx, f.hiredOffer.UserID, f.hiredOffer.OfferID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if offerStatus(t, db, f.hiredOffer.OfferID) != domain.OfferStatusPending {
		t.Fatal("canceled withdrawal changed the offer")
	}
}

func TestOfferWithdrawalWinsBeforeConcurrentAcceptance(t *testing.T) {
	f := newCommittedHiringFixture(t, "withdraw-before-accept")
	withdrawDB := openRepositoryIntegrationDB(t)
	acceptDB := openRepositoryIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Pause after UPDATE takes the offer lock but before its transaction commits.
	reached := pauseOnce(t, func(name string, fn func(*gorm.DB)) error {
		return withdrawDB.Callback().Update().After("gorm:update").Before("gorm:commit_or_rollback_transaction").Register(name, fn)
	}, "offers", "test:pause_withdrawal")
	withdrawErr := make(chan error, 1)
	go func() {
		_, err := NewMarketerJobRequestRepository(withdrawDB).WithdrawOffer(ctx, f.hiredID, f.hiredOffer.OfferID)
		withdrawErr <- err
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("withdrawal did not reach its locked update")
	}
	_, acceptErr := NewJobOfferRepository(acceptDB).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID)
	if err := <-withdrawErr; err != nil {
		t.Fatalf("withdrawal that locked first must succeed, got %v", err)
	}
	if !errors.Is(acceptErr, domain.ErrOfferNotPending) {
		t.Fatalf("acceptance must wait and reject the withdrawn offer, got %v", acceptErr)
	}
	if offerStatus(t, acceptDB, f.hiredOffer.OfferID) != domain.OfferStatusWithdrawn {
		t.Fatal("acceptance overwrote the withdrawn offer")
	}
	var job domain.Job
	if err := acceptDB.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusPending || job.AcceptedOfferID != nil || requestStatus(t, acceptDB, f.competitorReq) != domain.JobRequestStatusAccepted {
		t.Fatal("failed concurrent acceptance must leave the job and invitations unchanged")
	}
}

func TestOfferAcceptanceWinsBeforeConcurrentWithdrawal(t *testing.T) {
	f := newCommittedHiringFixture(t, "accept-before-withdraw")
	withdrawDB := openRepositoryIntegrationDB(t)
	acceptDB := openRepositoryIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Acceptance has already locked the job and selected offer at this point.
	reached := pauseOnce(t, func(name string, fn func(*gorm.DB)) error {
		return acceptDB.Callback().Update().Before("gorm:update").Register(name, fn)
	}, "offers", "test:pause_acceptance_before_withdrawal")
	acceptErr := make(chan error, 1)
	go func() {
		_, err := NewJobOfferRepository(acceptDB).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID)
		acceptErr <- err
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("acceptance did not reach its locked update")
	}
	_, withdrawErr := NewMarketerJobRequestRepository(withdrawDB).WithdrawOffer(ctx, f.hiredID, f.hiredOffer.OfferID)
	if err := <-acceptErr; err != nil {
		t.Fatalf("acceptance that locked first must succeed, got %v", err)
	}
	if !errors.Is(withdrawErr, domain.ErrOfferNotPending) {
		t.Fatalf("withdrawal must wait and reject the accepted offer, got %v", withdrawErr)
	}
	if offerStatus(t, withdrawDB, f.hiredOffer.OfferID) != domain.OfferStatusAccepted {
		t.Fatal("withdrawal overwrote the accepted offer")
	}
	var job domain.Job
	if err := withdrawDB.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusAccepted || job.AcceptedOfferID == nil || *job.AcceptedOfferID != f.hiredOffer.OfferID {
		t.Fatal("concurrent withdrawal changed the accepted job")
	}
}
