package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type fakeJobSettlementRepository struct {
	operation     string
	userID, jobID int32
	settlement    *domain.JobSettlement
	err           error
}

func (f *fakeJobSettlementRepository) GetPayment(_ context.Context, userID, jobID int32) (*domain.JobSettlement, error) {
	f.operation, f.userID, f.jobID = "payment", userID, jobID
	return f.settlement, f.err
}

func (f *fakeJobSettlementRepository) Complete(_ context.Context, userID, jobID int32) (*domain.JobSettlement, error) {
	f.operation, f.userID, f.jobID = "complete", userID, jobID
	return f.settlement, f.err
}

func TestJobSettlementServiceComplete(t *testing.T) {
	saved := &domain.JobSettlement{JobID: 12, JobStatus: domain.JobStatusCompleted}
	for _, tt := range []struct {
		name    string
		actor   int32
		creator bool
		repoErr error
		wantErr error
		called  bool
	}{
		{"anonymous", 0, true, nil, domain.ErrCreatorRequired, false},
		{"marketer only", 8, false, nil, domain.ErrCreatorRequired, false},
		{"foreign job", 8, true, domain.ErrJobForbidden, domain.ErrJobForbidden, true},
		{"payment conflict", 8, true, domain.ErrCompletionPaymentRequired, domain.ErrCompletionPaymentRequired, true},
		{"proof conflict", 8, true, domain.ErrCompletionProofRequired, domain.ErrCompletionProofRequired, true},
		{"saved", 8, true, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeJobSettlementRepository{err: tt.repoErr}
			if tt.repoErr == nil {
				repo.settlement = saved
			}
			service := NewJobSettlementService(repo, &fakeMembershipRepository{creator: tt.creator})
			got, err := service.Complete(context.Background(), Actor{UserID: tt.actor}, 12)
			if !errors.Is(err, tt.wantErr) || (err == nil && got != saved) {
				t.Fatalf("got %+v, %v, want %v", got, err, tt.wantErr)
			}
			if tt.called {
				if repo.operation != "complete" || repo.userID != tt.actor || repo.jobID != 12 {
					t.Fatalf("unexpected repository call: %+v", repo)
				}
			} else if repo.operation != "" {
				t.Fatalf("unexpected repository call: %+v", repo)
			}
		})
	}
}

func TestJobSettlementServiceReads(t *testing.T) {
	ctx := context.Background()
	actor := Actor{UserID: 8}
	repo := &fakeJobSettlementRepository{settlement: &domain.JobSettlement{JobID: 12}}
	marketer := NewJobSettlementService(repo, &fakeMembershipRepository{marketer: true})
	if _, err := marketer.GetPayment(ctx, Actor{}, 12); !errors.Is(err, domain.ErrJobForbidden) || repo.operation != "" {
		t.Fatalf("anonymous payment read: %v, repo %+v", err, repo)
	}
	if got, err := marketer.GetPayment(ctx, actor, 12); err != nil || got != repo.settlement || repo.operation != "payment" || repo.userID != 8 || repo.jobID != 12 {
		t.Fatalf("participant payment read: %+v, %v, repo %+v", got, err, repo)
	}
}
