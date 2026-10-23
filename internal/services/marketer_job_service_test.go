package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeMarketerJobRepository struct {
	startCalls int
	startActor int32
	startJob   int32
	startCtx   context.Context
	startErr   error
}

func (f *fakeMarketerJobRepository) StartJob(ctx context.Context, marketerID, jobID int32) (*ports.StartedJob, error) {
	f.startCalls++
	f.startActor, f.startJob, f.startCtx = marketerID, jobID, ctx
	if f.startErr != nil {
		return nil, f.startErr
	}
	offerID := int32(31)
	return &ports.StartedJob{
		Job:   domain.Job{JobID: jobID, JobStatus: domain.JobStatusInProgress, AcceptedOfferID: &offerID},
		Offer: domain.Offer{OfferID: offerID, JobID: jobID, UserID: marketerID, OfferStatus: domain.OfferStatusAccepted},
	}, nil
}

func TestMarketerJobServiceStartJobUsesActorAndContext(t *testing.T) {
	repo := &fakeMarketerJobRepository{}
	service := NewMarketerJobService(repo, &fakeMembershipRepository{marketer: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, err := service.StartJob(ctx, Actor{UserID: 9}, 12)
	if err != nil {
		t.Fatal(err)
	}
	if repo.startActor != 9 || repo.startJob != 12 || repo.startCtx != ctx || started.Job.JobStatus != domain.JobStatusInProgress {
		t.Fatalf("start must use the authenticated actor and request context: repo=%+v started=%+v", repo, started)
	}
}

func TestMarketerJobServiceStartJobRejectsBeforePersistence(t *testing.T) {
	for _, tt := range []struct {
		name     string
		actor    int32
		marketer bool
		jobID    int32
		want     error
	}{
		{"non_marketer", 9, false, 12, domain.ErrMarketerRequired},
		{"invalid_actor", 0, true, 12, domain.ErrMarketerRequired},
		{"zero_job", 9, true, 0, domain.ErrInvalidJob},
		{"negative_job", 9, true, -1, domain.ErrInvalidJob},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeMarketerJobRepository{}
			service := NewMarketerJobService(repo, &fakeMembershipRepository{marketer: tt.marketer})
			if _, err := service.StartJob(context.Background(), Actor{UserID: tt.actor}, tt.jobID); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if repo.startCalls != 0 {
				t.Fatal("invalid start reached persistence")
			}
		})
	}
}

func TestMarketerJobServiceStartJobPropagatesErrors(t *testing.T) {
	for _, want := range []error{
		domain.ErrJobNotFound, domain.ErrJobForbidden, domain.ErrJobNotAccepted,
		domain.ErrAcceptedOfferMismatch, context.Canceled, errors.New("database unavailable"),
	} {
		repo := &fakeMarketerJobRepository{startErr: want}
		service := NewMarketerJobService(repo, &fakeMembershipRepository{marketer: true})
		if _, err := service.StartJob(context.Background(), Actor{UserID: 9}, 12); !errors.Is(err, want) {
			t.Fatalf("expected %v, got %v", want, err)
		}
	}
}
