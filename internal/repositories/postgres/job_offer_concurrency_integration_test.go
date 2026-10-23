package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// These tests commit data and use separate connections, because lock
// ordering between two transactions cannot be observed inside the single
// rolled-back transaction the other repository tests share.

const concurrencyPause = 500 * time.Millisecond

// openRepositoryIntegrationDB opens its own connection pool to the disposable
// test database. Each caller gets an independent *gorm.DB, so callbacks
// registered on one do not affect the others.
func openRepositoryIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	if err := validateIntegrationDatabaseURL(databaseURL); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

type committedHiringFixture struct {
	creatorID     int32
	hiredID       int32
	competitorID  int32
	jobID         int32
	hiredOffer    *domain.Offer
	competitorReq int32
}

// newCommittedHiringFixture commits a Pending job where the hired marketer
// has a Pending offer and the competitor has accepted their request but not
// yet offered. Every row is deleted when the test ends.
func newCommittedHiringFixture(t *testing.T, label string) committedHiringFixture {
	t.Helper()
	db := openRepositoryIntegrationDB(t)
	ctx := context.Background()
	f := committedHiringFixture{
		creatorID:    createIntegrationCreator(t, db, label+"-creator"),
		hiredID:      createIntegrationMarketer(t, db, label+"-hired"),
		competitorID: createIntegrationMarketer(t, db, label+"-competitor"),
	}
	t.Cleanup(func() { deleteCommittedHiringFixture(t, db, f) })
	surveyID := createIntegrationSurvey(t, db, f.creatorID, label)
	job, err := NewJobRepository(db).CreateWithSurveys(ctx, &domain.Job{UserID: f.creatorID, JobStatus: domain.JobStatusPending, CreatedAt: time.Now().UTC()}, []int32{surveyID})
	if err != nil {
		t.Fatal(err)
	}
	f.jobID = job.JobID
	requests, err := NewJobRepository(db).CreateRequests(ctx, f.creatorID, f.jobID, []int32{f.hiredID, f.competitorID}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		acceptIntegrationJobRequest(t, db, request.MarketerID, request.JobRequestID)
		if request.MarketerID == f.competitorID {
			f.competitorReq = request.JobRequestID
		}
	}
	f.hiredOffer = newIntegrationOffer("9000")
	if err := NewMarketerJobRequestRepository(db).CreateOffer(ctx, f.hiredID, requests[0].JobRequestID, f.hiredOffer); err != nil {
		t.Fatal(err)
	}
	return f
}

func deleteCommittedHiringFixture(t *testing.T, db *gorm.DB, f committedHiringFixture) {
	t.Helper()
	users := []int32{f.creatorID, f.hiredID, f.competitorID}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE jobs SET accepted_offer_id = NULL WHERE user_id = ?`, []any{f.creatorID}},
		{`DELETE FROM offers WHERE job_id IN (SELECT job_id FROM jobs WHERE user_id = ?)`, []any{f.creatorID}},
		{`DELETE FROM job_requests WHERE job_id IN (SELECT job_id FROM jobs WHERE user_id = ?)`, []any{f.creatorID}},
		{`DELETE FROM is_used_in WHERE job_id IN (SELECT job_id FROM jobs WHERE user_id = ?)`, []any{f.creatorID}},
		{`DELETE FROM jobs WHERE user_id = ?`, []any{f.creatorID}},
		{`DELETE FROM surveys WHERE user_id = ?`, []any{f.creatorID}},
		{`DELETE FROM creators WHERE user_id = ?`, []any{f.creatorID}},
		{`DELETE FROM marketers WHERE user_id IN ?`, []any{users}},
		{`DELETE FROM users WHERE user_id IN ?`, []any{users}},
	} {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Errorf("clean up concurrency fixture: %v", err)
		}
	}
}

// pauseOnce registers a callback through register that, the first time it
// runs for table, signals reached and then holds the transaction open for
// concurrencyPause.
func pauseOnce(t *testing.T, register func(name string, fn func(*gorm.DB)) error, table, name string) <-chan struct{} {
	t.Helper()
	reached := make(chan struct{})
	var once sync.Once
	if err := register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != table {
			return
		}
		once.Do(func() {
			close(reached)
			time.Sleep(concurrencyPause)
		})
	}); err != nil {
		t.Fatal(err)
	}
	return reached
}

func TestOfferSubmissionAndAcceptanceDoNotDeadlockWhenSubmissionStartsFirst(t *testing.T) {
	f := newCommittedHiringFixture(t, "lock-order-submit-first")
	submitDB := openRepositoryIntegrationDB(t)
	acceptDB := openRepositoryIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Hold the submission open after its locks are taken and just before the
	// offer insert, then start the acceptance inside that window.
	reached := pauseOnce(t, func(name string, fn func(*gorm.DB)) error {
		return submitDB.Callback().Create().Before("gorm:create").Register(name, fn)
	}, "offers", "test:pause_offer_insert")
	competitorOffer := newIntegrationOffer("8000")
	submitErr := make(chan error, 1)
	go func() {
		submitErr <- NewMarketerJobRequestRepository(submitDB).CreateOffer(ctx, f.competitorID, f.competitorReq, competitorOffer)
	}()
	<-reached
	result, acceptErr := NewJobOfferRepository(acceptDB).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID)

	if err := <-submitErr; err != nil {
		t.Fatalf("submission that started first must succeed, got %v", err)
	}
	if acceptErr != nil {
		t.Fatalf("acceptance must wait for the submission instead of deadlocking, got %v", acceptErr)
	}
	if len(result.RejectedOfferIDs) != 1 || result.RejectedOfferIDs[0] != competitorOffer.OfferID {
		t.Fatalf("the concurrently submitted offer must be rejected, got %v", result.RejectedOfferIDs)
	}
	var pending int64
	if err := acceptDB.Model(&domain.Offer{}).Where("job_id = ? AND offer_status = ?", f.jobID, domain.OfferStatusPending).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("an accepted job must not keep %d pending offers", pending)
	}
}

func TestOfferSubmissionWaitsForAcceptanceAndIsRefusedWhenAcceptanceStartsFirst(t *testing.T) {
	f := newCommittedHiringFixture(t, "lock-order-accept-first")
	submitDB := openRepositoryIntegrationDB(t)
	acceptDB := openRepositoryIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Hold the acceptance open after it has locked the job and closed the
	// competing requests, then start the submission inside that window. The
	// paused jobs update is the one that fires the acceptance triggers.
	reached := pauseOnce(t, func(name string, fn func(*gorm.DB)) error {
		return acceptDB.Callback().Update().Before("gorm:update").Register(name, fn)
	}, "jobs", "test:pause_offer_accept")
	acceptErr := make(chan error, 1)
	go func() {
		_, err := NewJobOfferRepository(acceptDB).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID)
		acceptErr <- err
	}()
	<-reached
	submitErr := NewMarketerJobRequestRepository(submitDB).CreateOffer(ctx, f.competitorID, f.competitorReq, newIntegrationOffer("8000"))

	if err := <-acceptErr; err != nil {
		t.Fatalf("acceptance that started first must succeed, got %v", err)
	}
	if !errors.Is(submitErr, domain.ErrJobNotPending) {
		t.Fatalf("a submission after hiring must be refused with ErrJobNotPending, got %v", submitErr)
	}
}

func TestHiredJobRefusesNewInvitationsAndOffers(t *testing.T) {
	f := newCommittedHiringFixture(t, "hired-job-closed")
	db := openRepositoryIntegrationDB(t)
	ctx := context.Background()
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}

	// The reviewer's reproduction: re-invite the closed competitor after hiring.
	if _, err := NewJobRepository(db).CreateRequests(ctx, f.creatorID, f.jobID, []int32{f.competitorID}, time.Now().UTC()); !errors.Is(err, domain.ErrJobNotPending) {
		t.Fatalf("a hired job must refuse new invitations, got %v", err)
	}

	// Even if an Accepted request exists on a hired job, it cannot receive an offer.
	if _, err := NewMarketerJobRequestRepository(db).RespondToPending(ctx, f.competitorID, f.competitorReq, ports.JobRequestResponse{}); !errors.Is(err, domain.ErrJobRequestNotPending) {
		t.Fatalf("the competitor's request must be closed, got %v", err)
	}
	if err := db.Model(&domain.JobRequest{}).Where("job_request_id = ?", f.competitorReq).Update("request_status", domain.JobRequestStatusAccepted).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewMarketerJobRequestRepository(db).CreateOffer(ctx, f.competitorID, f.competitorReq, newIntegrationOffer("8000")); !errors.Is(err, domain.ErrJobNotPending) {
		t.Fatalf("a hired job must refuse new offers, got %v", err)
	}
}
