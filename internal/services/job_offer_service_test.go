package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeJobOfferRepository struct {
	listCalls int
	creatorID int32
	jobID     int32
	status    *domain.OfferStatus
	err       error
}

func (f *fakeJobOfferRepository) ListForCreatorJob(_ context.Context, creatorID, jobID int32, status *domain.OfferStatus) ([]ports.JobOfferComparison, error) {
	f.listCalls++
	f.creatorID = creatorID
	f.jobID = jobID
	f.status = status
	return []ports.JobOfferComparison{}, f.err
}

func TestJobOfferServiceListsOffersForTheActorsJob(t *testing.T) {
	repo := &fakeJobOfferRepository{}
	service := NewJobOfferService(repo, &fakeMembershipRepository{creator: true})
	status := domain.OfferStatusPending
	if _, err := service.ListOffers(context.Background(), Actor{UserID: 8}, 12, &status); err != nil {
		t.Fatal(err)
	}
	if repo.creatorID != 8 || repo.jobID != 12 || repo.status == nil || *repo.status != domain.OfferStatusPending {
		t.Fatalf("unexpected repository query: %+v", repo)
	}
}

func TestJobOfferServiceRejectsBeforePersistence(t *testing.T) {
	unknown := domain.OfferStatus("Archived")
	for name, tt := range map[string]struct {
		creator bool
		jobID   int32
		status  *domain.OfferStatus
		want    error
	}{
		"not_a_creator":  {creator: false, jobID: 12, want: domain.ErrCreatorRequired},
		"invalid_job":    {creator: true, jobID: 0, want: domain.ErrInvalidJob},
		"unknown_status": {creator: true, jobID: 12, status: &unknown, want: domain.ErrInvalidOffer},
	} {
		repo := &fakeJobOfferRepository{}
		service := NewJobOfferService(repo, &fakeMembershipRepository{creator: tt.creator})
		if _, err := service.ListOffers(context.Background(), Actor{UserID: 8}, tt.jobID, tt.status); !errors.Is(err, tt.want) {
			t.Fatalf("%s: expected %v, got %v", name, tt.want, err)
		}
		if repo.listCalls != 0 {
			t.Fatalf("%s: reached persistence", name)
		}
	}
}

func TestJobOfferServicePropagatesOwnershipErrors(t *testing.T) {
	for _, repoErr := range []error{domain.ErrJobForbidden, domain.ErrJobNotFound} {
		service := NewJobOfferService(&fakeJobOfferRepository{err: repoErr}, &fakeMembershipRepository{creator: true})
		if _, err := service.ListOffers(context.Background(), Actor{UserID: 8}, 12, nil); !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	}
}
