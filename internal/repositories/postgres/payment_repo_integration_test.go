package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newPayment() *domain.Payment {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Payment{Amount: decimal.RequireFromString("100.00"), Method: domain.PaymentMethodPromptPay, PaymentStatus: domain.PaymentStatusPending, CreatedAt: &now}
}

func TestPaymentRepositoryPersistsPendingPayments(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "payment-success")
	marketer := createIntegrationMarketer(t, db, "payment-success")
	for _, status := range []domain.JobStatus{domain.JobStatusAccepted, domain.JobStatusInProgress, domain.JobStatusCompleted} {
		for _, method := range []domain.PaymentMethod{domain.PaymentMethodPromptPay, domain.PaymentMethodBankTransfer, domain.PaymentMethodCreditCard} {
			jobID := createStatisticsJob(t, db, creator, marketer, status)
			input := newPayment()
			input.Method = method
			result, err := NewPaymentRepository(db).CreateForCreatorJob(context.Background(), creator, jobID, input)
			if err != nil {
				t.Fatal(err)
			}
			var stored domain.Payment
			if err := db.First(&stored, "payment_id = ?", result.PaymentID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.JobID != jobID || stored.Method != method || stored.Amount.StringFixed(2) != "100.00" || stored.PaymentStatus != domain.PaymentStatusPending || stored.PaidAt != nil || stored.CreatedAt == nil || !stored.CreatedAt.Equal(*input.CreatedAt) {
				t.Fatalf("unexpected stored payment: %+v", stored)
			}
			var job domain.Job
			if err := db.First(&job, "job_id = ?", jobID).Error; err != nil {
				t.Fatal(err)
			}
			if job.JobStatus != status {
				t.Fatalf("Job status changed to %s", job.JobStatus)
			}
		}
	}
}

func TestPaymentRepositoryRejectsWithoutWriting(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "payment-errors")
	other := createIntegrationCreator(t, db, "payment-foreign")
	marketer := createIntegrationMarketer(t, db, "payment-errors")
	for _, tc := range []struct {
		name             string
		status           domain.JobStatus
		foreign, noOffer bool
		amount           string
		active           domain.PaymentStatus
		want             error
	}{
		{name: "foreign", status: domain.JobStatusAccepted, foreign: true, want: domain.ErrJobForbidden},
		{name: "pending_job", status: domain.JobStatusPending, want: domain.ErrJobNotPayable},
		{name: "cancelled_job", status: domain.JobStatusCancelled, want: domain.ErrJobNotPayable},
		{name: "no_offer", status: domain.JobStatusAccepted, noOffer: true, want: domain.ErrAcceptedOfferRequired},
		{name: "underpayment", status: domain.JobStatusAccepted, amount: "99.99", want: domain.ErrPaymentAmountMismatch},
		{name: "overpayment", status: domain.JobStatusAccepted, amount: "100.01", want: domain.ErrPaymentAmountMismatch},
		{name: "pending_payment", status: domain.JobStatusAccepted, active: domain.PaymentStatusPending, want: domain.ErrPaymentAlreadyExists},
		{name: "paid_payment", status: domain.JobStatusAccepted, active: domain.PaymentStatusPaid, want: domain.ErrPaymentAlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobID := createStatisticsJob(t, db, creator, marketer, tc.status)
			if tc.noOffer {
				if err := db.Exec("UPDATE jobs SET accepted_offer_id = NULL WHERE job_id = ?", jobID).Error; err != nil {
					t.Fatal(err)
				}
			}
			before := int64(0)
			if tc.active != "" {
				createStatisticsPayment(t, db, jobID, "100", tc.active)
				before = 1
			}
			actor := creator
			if tc.foreign {
				actor = other
			}
			payment := newPayment()
			if tc.amount != "" {
				payment.Amount = decimal.RequireFromString(tc.amount)
			}
			_, err := NewPaymentRepository(db).CreateForCreatorJob(context.Background(), actor, jobID, payment)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			var after int64
			if err := db.Model(&domain.Payment{}).Where("job_id = ?", jobID).Count(&after).Error; err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatalf("failed request wrote a payment: before=%d after=%d", before, after)
			}
		})
	}
	if _, err := NewPaymentRepository(db).CreateForCreatorJob(context.Background(), creator, 0, newPayment()); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("missing Job: %v", err)
	}
}

func TestPaymentRepositoryRetainsFailedAndRefundedHistory(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	creator := createIntegrationCreator(t, db, "payment-retry")
	marketer := createIntegrationMarketer(t, db, "payment-retry")
	jobID := createStatisticsJob(t, db, creator, marketer, domain.JobStatusAccepted)
	createStatisticsPayment(t, db, jobID, "100", domain.PaymentStatusFailed)
	createStatisticsPayment(t, db, jobID, "100", domain.PaymentStatusRefunded)
	if _, err := NewPaymentRepository(db).CreateForCreatorJob(context.Background(), creator, jobID, newPayment()); err != nil {
		t.Fatal(err)
	}
	var job domain.Job
	if err := db.Preload("Payments").First(&job, "job_id = ?", jobID).Error; err != nil {
		t.Fatal(err)
	}
	if len(job.Payments) != 3 {
		t.Fatalf("expected three historical payments, got %d", len(job.Payments))
	}
}

func TestConcurrentPaymentsCreateOnlyOneRecord(t *testing.T) {
	f := newCommittedHiringFixture(t, "payment-concurrent")
	db1 := openRepositoryIntegrationDB(t)
	db2 := openRepositoryIntegrationDB(t)
	t.Cleanup(func() {
		if err := db1.Exec("DELETE FROM payments WHERE job_id = ?", f.jobID).Error; err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := NewJobOfferRepository(db1).AcceptOffer(ctx, f.creatorID, f.jobID, f.hiredOffer.OfferID); err != nil {
		t.Fatal(err)
	}
	reached := pauseOnce(t, func(name string, fn func(*gorm.DB)) error {
		return db1.Callback().Create().Before("gorm:create").Register(name, fn)
	}, "payments", "test:pause_payment_insert")
	results := make(chan error, 2)
	submit := func(db *gorm.DB) {
		p := newPayment()
		p.Amount = decimal.RequireFromString("9000")
		_, err := NewPaymentRepository(db).CreateForCreatorJob(ctx, f.creatorID, f.jobID, p)
		results <- err
	}
	go submit(db1)
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("first payment did not reach insert")
	}
	go submit(db2)
	success, conflict := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrPaymentAlreadyExists) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	var count int64
	if err := db1.Model(&domain.Payment{}).Where("job_id = ?", f.jobID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one payment, got %d", count)
	}
}

func TestPaymentMigrationPreservesLegacyRows(t *testing.T) {
	db := openRepositoryIntegrationTransaction(t)
	read := func(direction string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000011_payment_creation."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := db.Exec(read("down")).Error; err != nil {
		t.Fatal(err)
	}
	creator := createIntegrationCreator(t, db, "payment-migration")
	marketer := createIntegrationMarketer(t, db, "payment-migration")
	jobID := createStatisticsJob(t, db, creator, marketer, domain.JobStatusAccepted)
	var id int32
	if err := db.Raw("INSERT INTO payments (job_id,amount,method,payment_status) VALUES (?,100,'Other','Failed') RETURNING payment_id", jobID).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(read("up")).Error; err != nil {
		t.Fatal(err)
	}
	var legacy domain.Payment
	if err := db.First(&legacy, "payment_id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.Method != domain.PaymentMethodOther || legacy.PaymentStatus != domain.PaymentStatusFailed || legacy.Amount.StringFixed(2) != "100.00" || legacy.CreatedAt != nil {
		t.Fatalf("legacy data changed: %+v", legacy)
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO payments (job_id,amount,method,payment_status) VALUES (?,100,'Other','Pending')", jobID).Error
	})
	if err == nil {
		t.Fatal("new Other payment was accepted")
	}
	if err := db.Exec(read("down")).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO payments (job_id,amount,method,payment_status) VALUES (?,100,'Other','Pending')", jobID).Error; err != nil {
		t.Fatalf("down migration did not restore Other: %v", err)
	}
}
