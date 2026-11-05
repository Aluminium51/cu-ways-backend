package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

func TestMarketerReviewServiceListsForCreatorAndAppliesDefaults(t *testing.T) {
	average := 4.5
	repo := &fakeReviewRepository{listPage: ports.MarketerReviewPage{
		Items:         []domain.Review{{ReviewID: 9, JobID: 12, Rating: 5}},
		AverageRating: &average,
		Total:         1,
		Page:          DefaultMarketerReviewPage,
		PageSize:      DefaultMarketerReviewPageSize,
	}}
	profiles := &fakeMarketerProfileRepository{profile: &domain.Marketer{UserID: 18}}
	service := NewMarketerReviewService(repo, profiles, &fakeMembershipRepository{creator: true})

	page, err := service.List(context.Background(), Actor{UserID: 7}, 18, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if repo.listCalls != 1 || repo.listQuery.MarketerID != 18 {
		t.Fatalf("expected one list call scoped to marketer 18, got %+v", repo.listQuery)
	}
	if repo.listQuery.Page != DefaultMarketerReviewPage || repo.listQuery.PageSize != DefaultMarketerReviewPageSize {
		t.Fatalf("expected default paging, got page=%d size=%d", repo.listQuery.Page, repo.listQuery.PageSize)
	}
	if page.Total != 1 || page.AverageRating == nil || *page.AverageRating != 4.5 {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestMarketerReviewServiceClampsPageSizeAndAllowsAdmin(t *testing.T) {
	repo := &fakeReviewRepository{}
	profiles := &fakeMarketerProfileRepository{profile: &domain.Marketer{UserID: 18}}
	// Admin skips the Creator check; membership repo would report false.
	service := NewMarketerReviewService(repo, profiles, &fakeMembershipRepository{})

	if _, err := service.List(context.Background(), Actor{UserID: 1, IsAdmin: true}, 18, 3, MaxMarketerReviewPageSize+50); err != nil {
		t.Fatal(err)
	}
	if repo.listQuery.Page != 3 || repo.listQuery.PageSize != MaxMarketerReviewPageSize {
		t.Fatalf("expected page 3 and clamped size %d, got page=%d size=%d", MaxMarketerReviewPageSize, repo.listQuery.Page, repo.listQuery.PageSize)
	}
}

func TestMarketerReviewServiceReturnsEmptyStateWithoutFabricatedRating(t *testing.T) {
	repo := &fakeReviewRepository{listPage: ports.MarketerReviewPage{Items: []domain.Review{}, AverageRating: nil, Total: 0, Page: 1, PageSize: 10}}
	profiles := &fakeMarketerProfileRepository{profile: &domain.Marketer{UserID: 18}}
	service := NewMarketerReviewService(repo, profiles, &fakeMembershipRepository{creator: true})

	page, err := service.List(context.Background(), Actor{UserID: 7}, 18, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || page.AverageRating != nil || len(page.Items) != 0 {
		t.Fatalf("expected empty state with nil average, got %+v", page)
	}
}

func TestMarketerReviewServiceRejectsBeforeListing(t *testing.T) {
	for _, tc := range []struct {
		name       string
		actor      Actor
		marketerID int32
		creator    bool
		missing    bool
		want       error
	}{
		{name: "anonymous", actor: Actor{}, marketerID: 18, want: domain.ErrMarketerProfileNotFound},
		{name: "invalid_marketer_id", actor: Actor{UserID: 7}, marketerID: 0, creator: true, want: domain.ErrMarketerProfileNotFound},
		{name: "not_creator", actor: Actor{UserID: 7}, marketerID: 18, want: domain.ErrCreatorRequired},
		{name: "marketer_not_found", actor: Actor{UserID: 7}, marketerID: 18, creator: true, missing: true, want: domain.ErrMarketerProfileNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeReviewRepository{}
			profiles := &fakeMarketerProfileRepository{}
			if !tc.missing {
				profiles.profile = &domain.Marketer{UserID: tc.marketerID}
			}
			service := NewMarketerReviewService(repo, profiles, &fakeMembershipRepository{creator: tc.creator})

			_, err := service.List(context.Background(), tc.actor, tc.marketerID, 1, 10)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			if repo.listCalls != 0 {
				t.Fatalf("expected no listing when rejected, got %d calls", repo.listCalls)
			}
		})
	}
}
