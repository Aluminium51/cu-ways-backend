package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakePaymentRepository struct {
	creatorID, jobID int32
	calls            int
	payment          *domain.Payment
	err              error
}

func (f *fakePaymentRepository) CreateForCreatorJob(_ context.Context, creatorID, jobID int32, p *domain.Payment) (*domain.Payment, error) {
	f.calls++
	f.creatorID = creatorID
	f.jobID = jobID
	f.payment = p
	if f.err != nil {
		return nil, f.err
	}
	p.PaymentID = 3
	return p, nil
}

func TestPaymentServiceCreatesPendingPayment(t *testing.T) {
	for _, method := range []domain.PaymentMethod{domain.PaymentMethodPromptPay, domain.PaymentMethodBankTransfer, domain.PaymentMethodCreditCard} {
		repo := &fakePaymentRepository{}
		before := time.Now().UTC()
		p, err := NewPaymentService(repo, &fakeMembershipRepository{creator: true}).Create(context.Background(), Actor{UserID: 8}, 12, CreatePaymentInput{Method: method, Amount: "1500.00"})
		if err != nil {
			t.Fatal(err)
		}
		if repo.creatorID != 8 || repo.jobID != 12 || p.PaymentID != 3 || p.JobID != 12 || p.Amount.StringFixed(2) != "1500.00" || p.Method != method || p.PaymentStatus != domain.PaymentStatusPending || p.PaidAt != nil || p.CreatedAt == nil || p.CreatedAt.Before(before) || p.CreatedAt.After(time.Now().UTC()) {
			t.Fatalf("unexpected payment: %+v", p)
		}
	}
}

func TestPaymentServiceRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name       string
		creator    bool
		actor, job int32
		method     domain.PaymentMethod
		amount     string
		want       error
	}{
		{"not_creator", false, 8, 12, domain.PaymentMethodPromptPay, "100", domain.ErrCreatorRequired},
		{"invalid_actor", true, 0, 12, domain.PaymentMethodPromptPay, "100", domain.ErrCreatorRequired},
		{"invalid_job", true, 8, 0, domain.PaymentMethodPromptPay, "100", domain.ErrInvalidJob},
		{"other", true, 8, 12, domain.PaymentMethodOther, "100", domain.ErrInvalidPayment},
		{"missing_method", true, 8, 12, "", "100", domain.ErrInvalidPayment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakePaymentRepository{}
			_, err := NewPaymentService(repo, &fakeMembershipRepository{creator: tc.creator}).Create(context.Background(), Actor{UserID: tc.actor}, tc.job, CreatePaymentInput{Method: tc.method, Amount: tc.amount})
			if !errors.Is(err, tc.want) || repo.calls != 0 {
				t.Fatalf("got %v, calls %d", err, repo.calls)
			}
		})
	}
	for _, amount := range []string{"", "-1", "1.001", "100000000", "invalid"} {
		repo := &fakePaymentRepository{}
		_, err := NewPaymentService(repo, &fakeMembershipRepository{creator: true}).Create(context.Background(), Actor{UserID: 8}, 12, CreatePaymentInput{Method: domain.PaymentMethodPromptPay, Amount: amount})
		if !errors.Is(err, domain.ErrInvalidPayment) || repo.calls != 0 {
			t.Fatalf("amount %q: %v, calls %d", amount, err, repo.calls)
		}
	}
}

func TestPaymentServicePreservesZeroAndRepositoryErrors(t *testing.T) {
	repo := &fakePaymentRepository{}
	p, err := NewPaymentService(repo, &fakeMembershipRepository{creator: true}).Create(context.Background(), Actor{UserID: 8}, 12, CreatePaymentInput{Method: domain.PaymentMethodPromptPay, Amount: "0"})
	if err != nil || !p.Amount.IsZero() {
		t.Fatalf("zero payment: %v %v", p, err)
	}
	for _, want := range []error{domain.ErrJobNotFound, domain.ErrJobForbidden, domain.ErrJobNotPayable, domain.ErrAcceptedOfferRequired, domain.ErrPaymentAmountMismatch, domain.ErrPaymentAlreadyExists} {
		repo.err = want
		_, err := NewPaymentService(repo, &fakeMembershipRepository{creator: true}).Create(context.Background(), Actor{UserID: 8}, 12, CreatePaymentInput{Method: domain.PaymentMethodPromptPay, Amount: "100"})
		if !errors.Is(err, want) {
			t.Fatalf("expected %v, got %v", want, err)
		}
	}
}
