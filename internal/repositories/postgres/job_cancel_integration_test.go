package postgres

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func insertIntegrationPayment(t *testing.T, db *gorm.DB, jobID int32, status domain.PaymentStatus) int32 {
	t.Helper()
	var paymentID int32
	if err := db.Raw(`
INSERT INTO payments (job_id, amount, method, payment_status, paid_at, created_at)
VALUES (?, 9000, 'PromptPay', ?, NOW(), NOW())
RETURNING payment_id`, jobID, status).Scan(&paymentID).Error; err != nil {
		t.Fatal(err)
	}
	return paymentID
}

func TestJobRepositoryCancelClosesRequestsAndRejectsPendingOffers(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "cancel-happy")
	reason := "Survey no longer needed"
	cancelledAt := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	result, err := NewJobRepository(db).Cancel(context.Background(), f.creatorID, f.jobID, &reason, cancelledAt)
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.JobStatus != domain.JobStatusCancelled || result.Job.CancelledAt == nil || !result.Job.CancelledAt.Equal(cancelledAt) {
		t.Fatalf("unexpected job result: %+v", result.Job)
	}
	wantRejected := []int32{f.hiredOffer.OfferID, f.competingOffer.OfferID}
	slices.Sort(wantRejected)
	if !slices.Equal(result.RejectedOfferIDs, wantRejected) {
		t.Fatalf("expected offers %v to be rejected, got %v", wantRejected, result.RejectedOfferIDs)
	}
	wantClosed := []int32{f.hiredRequest, f.competingReq, f.pendingReq}
	slices.Sort(wantClosed)
	if !slices.Equal(result.ClosedJobRequestIDs, wantClosed) {
		t.Fatalf("expected requests %v to close, got %v", wantClosed, result.ClosedJobRequestIDs)
	}

	var job domain.Job
	if err := db.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.JobStatus != domain.JobStatusCancelled || job.CancelledAt == nil || !job.CancelledAt.Equal(cancelledAt) ||
		job.CancelReason == nil || *job.CancelReason != reason || job.AcceptedOfferID != nil {
		t.Fatalf("job was not persisted as cancelled: %+v", job)
	}
	for _, offerID := range wantRejected {
		if got := offerStatus(t, db, offerID); got != domain.OfferStatusRejected {
			t.Fatalf("offer %d is %s", offerID, got)
		}
	}
	for requestID, want := range map[int32]domain.JobRequestStatus{
		f.hiredRequest: domain.JobRequestStatusClosed,
		f.competingReq: domain.JobRequestStatusClosed,
		f.pendingReq:   domain.JobRequestStatusClosed,
		f.declinedReq:  domain.JobRequestStatusDeclined,
	} {
		if got := requestStatus(t, db, requestID); got != want {
			t.Fatalf("request %d is %s, want %s", requestID, got, want)
		}
	}
	summary, err := NewJobRepository(db).GetJobByCreator(context.Background(), f.creatorID, f.jobID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.CancelledAt == nil || summary.CancelReason == nil || *summary.CancelReason != reason || summary.PendingOfferCount != 0 {
		t.Fatalf("job summary does not show the cancellation: %+v", summary)
	}
}

func TestJobRepositoryCancelRefusesInvalidCancellations(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	ctx := context.Background()
	repo := NewJobRepository(db)
	f := newOfferAcceptanceFixture(t, db, "cancel-guard")
	other := newOfferAcceptanceFixture(t, db, "cancel-guard-other")
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	if _, err := repo.Cancel(ctx, other.creatorID, f.jobID, nil, now); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("expected another creator to be forbidden, got %v", err)
	}
	if _, err := repo.Cancel(ctx, f.creatorID, f.jobID+100000, nil, now); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("expected a missing job to be not found, got %v", err)
	}

	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, other.creatorID, other.jobID, other.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	if result, err := repo.Cancel(ctx, other.creatorID, other.jobID, nil, now); err != nil || result.Job.JobStatus != domain.JobStatusCancelled || result.Job.AcceptedOfferID == nil {
		t.Fatalf("expected unpaid accepted job to cancel and retain its accepted offer, got %+v, %v", result, err)
	}

	if _, err := repo.Cancel(ctx, f.creatorID, f.jobID, nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Cancel(ctx, f.creatorID, f.jobID, nil, now.Add(time.Hour)); !errors.Is(err, domain.ErrJobAlreadyCancelled) {
		t.Fatalf("expected a second cancellation to be refused, got %v", err)
	}
	var job domain.Job
	if err := db.First(&job, "job_id = ?", f.jobID).Error; err != nil {
		t.Fatal(err)
	}
	if job.CancelledAt == nil || !job.CancelledAt.Equal(now) {
		t.Fatalf("a refused cancellation overwrote cancelled_at: %v", job.CancelledAt)
	}
}

func TestJobRepositoryCancelRefusesPaidJobAndRollsBack(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "cancel-paid")
	insertIntegrationPayment(t, db, f.jobID, domain.PaymentStatusPaid)

	if _, err := NewJobRepository(db).Cancel(context.Background(), f.creatorID, f.jobID, nil, time.Now().UTC()); !errors.Is(err, domain.ErrJobHasPaidPayment) {
		t.Fatalf("expected a paid job to refuse cancellation, got %v", err)
	}
	if status, _ := jobStatusOf(t, db, f.jobID); status != domain.JobStatusPending {
		t.Fatalf("a refused cancellation changed the job to %s", status)
	}
	if got := offerStatus(t, db, f.competingOffer.OfferID); got != domain.OfferStatusPending {
		t.Fatalf("a refused cancellation changed an offer to %s", got)
	}
	if got := requestStatus(t, db, f.pendingReq); got != domain.JobRequestStatusPending {
		t.Fatalf("a refused cancellation changed a request to %s", got)
	}
}

func TestJobRepositoryCancelLeavesNonPaidPaymentsUnchanged(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "cancel-unpaid")
	pending := insertIntegrationPayment(t, db, f.jobID, domain.PaymentStatusPending)
	refunded := insertIntegrationPayment(t, db, f.jobID, domain.PaymentStatusRefunded)

	if _, err := NewJobRepository(db).Cancel(context.Background(), f.creatorID, f.jobID, nil, time.Now().UTC()); err != nil {
		t.Fatalf("expected Pending and Refunded payments to allow cancellation, got %v", err)
	}
	for paymentID, want := range map[int32]domain.PaymentStatus{pending: domain.PaymentStatusPending, refunded: domain.PaymentStatusRefunded} {
		var payment domain.Payment
		if err := db.First(&payment, "payment_id = ?", paymentID).Error; err != nil {
			t.Fatal(err)
		}
		if payment.PaymentStatus != want {
			t.Fatalf("payment %d is %s, want %s", paymentID, payment.PaymentStatus, want)
		}
	}
}

func TestJobCancellationConstraintRequiresCancelledStatus(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	f := newOfferAcceptanceFixture(t, db, "cancel-constraint")

	for name, sql := range map[string]string{
		"cancelled_at": `UPDATE jobs SET cancelled_at = NOW() WHERE job_id = ?`,
		"reason":       `UPDATE jobs SET cancel_reason = 'not cancelled' WHERE job_id = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			err := execGuarded(t, db, sql, f.jobID)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "chk_job_cancellation" {
				t.Fatalf("expected chk_job_cancellation to refuse the update, got %v", err)
			}
		})
	}
	// Jobs cancelled before the columns existed have no recorded time or reason.
	if err := execGuarded(t, db, `UPDATE jobs SET job_status = 'Cancelled' WHERE job_id = ?`, f.jobID); err != nil {
		t.Fatalf("a Cancelled job without details should be allowed, got %v", err)
	}
}
