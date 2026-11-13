package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
)

func TestJobSettlementLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "settlement-creator")
	marketer := createIntegrationMarketer(t, db, "settlement-marketer")
	stranger := createIntegrationCreator(t, db, "settlement-stranger")
	repo := NewJobSettlementRepository(db)
	jobID := createTransactionJob(t, db, creator, marketer, domain.JobStatusAccepted, "Settlement", "", "", time.Time{})
	if _, err := repo.GetPayment(ctx, stranger, jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("foreign payment read: %v", err)
	}
	for _, userID := range []int32{creator, marketer} {
		record, err := repo.GetPayment(ctx, userID, jobID)
		if err != nil || record.Payment != nil || record.CreatorID != creator || record.MarketerID != marketer || record.AcceptedPrice.StringFixed(2) != "100.00" {
			t.Fatalf("unpaid settlement for %d: %+v, %v", userID, record, err)
		}
	}
	if _, err := repo.Complete(ctx, marketer, jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("marketer completion: %v", err)
	}
	if _, err := repo.Complete(ctx, creator, jobID); !errors.Is(err, domain.ErrJobNotInProgress) {
		t.Fatalf("completion before start: %v", err)
	}
	if err := db.Exec("UPDATE jobs SET job_status = 'In Progress' WHERE job_id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	createStatisticsPayment(t, db, jobID, "95.00", domain.PaymentStatusPaid)
	if _, err := repo.Complete(ctx, creator, jobID); !errors.Is(err, domain.ErrCompletionPaymentRequired) {
		t.Fatalf("completion with mismatched payment: %v", err)
	}
	if err := db.Exec("UPDATE payments SET payment_status = 'Failed' WHERE job_id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	payment := &domain.Payment{
		Amount: decimal.NewFromInt(100), Method: domain.PaymentMethodPromptPay,
		PaymentStatus: domain.PaymentStatusPaid, PaidAt: &now, CreatedAt: &now,
	}
	payments := NewPaymentRepository(db)
	if _, err := payments.CreateForCreatorJob(ctx, creator, jobID, payment); err != nil {
		t.Fatal(err)
	}
	if _, err := payments.CreateForCreatorJob(ctx, creator, jobID, payment); !errors.Is(err, domain.ErrPaymentAlreadyExists) {
		t.Fatalf("duplicate payment: %v", err)
	}
	held, err := repo.GetPayment(ctx, marketer, jobID)
	if err != nil || held.PayoutStatus() != domain.PayoutStatusHeld || held.Payment.PaymentID != payment.PaymentID {
		t.Fatalf("held settlement: %+v, %v", held, err)
	}
	if _, err := repo.Complete(ctx, creator, jobID); !errors.Is(err, domain.ErrCompletionProofRequired) {
		t.Fatalf("completion without proof: %v", err)
	}
	if err := NewProofRepository(db).CreateForAcceptedMarketer(ctx, marketer, jobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeProof, URL: "https://example.test/delivery", UploadAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	completed, err := repo.Complete(ctx, creator, jobID)
	if err != nil || completed.CompletedAt == nil || completed.PayoutStatus() != domain.PayoutStatusReleased {
		t.Fatalf("completed settlement: %+v, %v", completed, err)
	}
	repeated, err := repo.Complete(ctx, creator, jobID)
	if err != nil || repeated.CompletedAt == nil || !repeated.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("completion retry: %+v, %v", repeated, err)
	}
	reviews := NewReviewRepository(db)
	if review, err := reviews.FindForCreatorJob(ctx, creator, jobID); err != nil || review != nil {
		t.Fatalf("review before submission: %+v, %v", review, err)
	}
	if _, err := reviews.CreateForCreatorJob(ctx, creator, jobID, &domain.Review{Rating: 5, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := reviews.CreateForCreatorJob(ctx, creator, jobID, &domain.Review{Rating: 3, CreatedAt: now}); !errors.Is(err, domain.ErrReviewAlreadyExists) {
		t.Fatalf("duplicate review: %v", err)
	}
	if _, err := reviews.FindForCreatorJob(ctx, marketer, jobID); !errors.Is(err, domain.ErrJobForbidden) {
		t.Fatalf("marketer review read: %v", err)
	}
	if review, err := reviews.FindForCreatorJob(ctx, creator, jobID); err != nil || review == nil || review.Rating != 5 {
		t.Fatalf("saved review: %+v, %v", review, err)
	}
	creatorHistory := listTransactions(t, db, ports.TransactionQuery{UserID: creator, Role: domain.TransactionRoleCreator})
	marketerHistory := listTransactions(t, db, ports.TransactionQuery{UserID: marketer, Role: domain.TransactionRoleMarketer})
	if creatorHistory.Total != 1 || creatorHistory.Items[0].HasReview == nil || !*creatorHistory.Items[0].HasReview || !creatorHistory.Items[0].CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("creator history: %+v", creatorHistory)
	}
	if marketerHistory.Total != 1 || marketerHistory.Items[0].HasReview != nil {
		t.Fatalf("marketer history: %+v", marketerHistory)
	}
}

func TestConcurrentJobCompletionReturnsOneTimestamp(t *testing.T) {
	ctx := context.Background()
	fixture := newCommittedHiringFixture(t, "settlement-concurrent")
	db := openRepositoryIntegrationDB(t)
	t.Cleanup(func() {
		for _, table := range []string{"payments", "attachments"} {
			if err := db.Table(table).Where("job_id = ?", fixture.jobID).Delete(&struct{}{}).Error; err != nil {
				t.Errorf("clean up %s: %v", table, err)
			}
		}
	})
	now := time.Now().UTC()
	if _, err := NewJobOfferRepository(db).AcceptOffer(ctx, fixture.creatorID, fixture.jobID, fixture.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE jobs SET job_status = 'In Progress' WHERE job_id = ?", fixture.jobID).Error; err != nil {
		t.Fatal(err)
	}
	createStatisticsPayment(t, db, fixture.jobID, "9000.00", domain.PaymentStatusPaid)
	if err := NewProofRepository(db).CreateForAcceptedMarketer(ctx, fixture.hiredID, fixture.jobID, &domain.Attachment{
		AttachmentType: domain.AttachmentTypeProof, URL: "https://example.test/delivery", UploadAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	type result struct {
		record *domain.JobSettlement
		err    error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	repo := NewJobSettlementRepository(db)
	for range 2 {
		go func() {
			<-start
			record, err := repo.Complete(ctx, fixture.creatorID, fixture.jobID)
			results <- result{record, err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.record.CompletedAt == nil || second.record.CompletedAt == nil ||
		!first.record.CompletedAt.Equal(*second.record.CompletedAt) {
		t.Fatalf("completion results: %+v and %+v", first, second)
	}
}
