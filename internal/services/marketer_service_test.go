package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
)

type fakeMembershipRepository struct {
	creator  bool
	marketer bool
}

func (f *fakeMembershipRepository) IsCreator(context.Context, int32) (bool, error) {
	return f.creator, nil
}

func (f *fakeMembershipRepository) IsMarketer(context.Context, int32) (bool, error) {
	return f.marketer, nil
}

type fakeCatalogRepository struct {
	expertise []domain.Expertise
	campuses  []domain.Campus
}

func (f *fakeCatalogRepository) FindExpertiseBySlugs(_ context.Context, slugs []string) ([]domain.Expertise, error) {
	result := make([]domain.Expertise, 0, len(slugs))
	for _, slug := range slugs {
		for _, option := range f.expertise {
			if option.Slug == slug {
				result = append(result, option)
			}
		}
	}
	return result, nil
}

func (f *fakeCatalogRepository) FindCampusesBySlugs(_ context.Context, slugs []string) ([]domain.Campus, error) {
	result := make([]domain.Campus, 0, len(slugs))
	for _, slug := range slugs {
		for _, option := range f.campuses {
			if option.Slug == slug {
				result = append(result, option)
			}
		}
	}
	return result, nil
}

type fakeMarketerProfileRepository struct {
	profile        *domain.Marketer
	searchQuery    ports.MarketerSearchQuery
	searchResult   ports.MarketerPage
	savedExpertise []int32
	savedCampuses  []int32
}

func (f *fakeMarketerProfileRepository) FindProfile(context.Context, int32) (*domain.Marketer, error) {
	if f.profile == nil {
		return nil, domain.ErrMarketerProfileNotFound
	}
	copy := *f.profile
	return &copy, nil
}

func (f *fakeMarketerProfileRepository) SaveProfile(_ context.Context, profile *domain.Marketer, expertiseIDs, campusIDs []int32) error {
	copy := *profile
	f.profile = &copy
	f.savedExpertise = expertiseIDs
	f.savedCampuses = campusIDs
	return nil
}

func (f *fakeMarketerProfileRepository) Search(_ context.Context, query ports.MarketerSearchQuery) (ports.MarketerPage, error) {
	f.searchQuery = query
	return f.searchResult, nil
}

func TestMarketerServiceSavesRequiredProfileAndNormalizesCatalogs(t *testing.T) {
	repo := &fakeMarketerProfileRepository{}
	catalog := &fakeCatalogRepository{
		expertise: []domain.Expertise{{ExpertiseID: 1, Slug: "data-collection", Name: "Data Collection"}},
		campuses:  []domain.Campus{{CampusID: 2, Slug: "cu-main-campus", Name: "CU Main Campus"}},
	}
	service := NewMarketerService(repo, catalog, &fakeMembershipRepository{})

	profile, err := service.SaveProfile(context.Background(), Actor{UserID: 7}, MarketerProfileInput{
		Bio:                "  Research specialist  ",
		ExperienceYears:    4,
		AvailabilityStatus: " AVAILABLE ",
		AvailabilityText:   " Weekdays ",
		ExpertiseSlugs:     []string{"DATA-COLLECTION", "data-collection"},
		CampusSlugs:        []string{"CU-MAIN-CAMPUS"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Bio != "Research specialist" || profile.AvailabilityStatus != domain.AvailabilityAvailable || profile.ExperienceYears != 4 {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if len(repo.savedExpertise) != 1 || repo.savedExpertise[0] != 1 || len(repo.savedCampuses) != 1 || repo.savedCampuses[0] != 2 {
		t.Fatalf("unexpected catalog IDs: expertise=%v campuses=%v", repo.savedExpertise, repo.savedCampuses)
	}
}

func TestMarketerServiceRejectsEmptyCoreProfileAndUnknownCatalog(t *testing.T) {
	service := NewMarketerService(&fakeMarketerProfileRepository{}, &fakeCatalogRepository{}, &fakeMembershipRepository{})
	_, err := service.SaveProfile(context.Background(), Actor{UserID: 1}, MarketerProfileInput{ExperienceYears: 1, AvailabilityStatus: "available"})
	if !errors.Is(err, domain.ErrInvalidMarketerProfile) {
		t.Fatalf("expected invalid profile, got %v", err)
	}
	_, err = service.SaveProfile(context.Background(), Actor{UserID: 1}, MarketerProfileInput{
		Bio: "bio", ExperienceYears: 1, AvailabilityStatus: "available", AvailabilityText: "now", ExpertiseSlugs: []string{"missing"},
	})
	if !errors.Is(err, domain.ErrCatalogOptionNotFound) {
		t.Fatalf("expected unknown catalog error, got %v", err)
	}
}

func TestMarketerServiceSearchRequiresCreatorAndAppliesDefaults(t *testing.T) {
	repo := &fakeMarketerProfileRepository{searchResult: ports.MarketerPage{Page: 1, PageSize: 20}}
	service := NewMarketerService(repo, &fakeCatalogRepository{}, &fakeMembershipRepository{creator: true})
	minPrice := decimal.NewFromInt(500)
	page, err := service.Search(context.Background(), Actor{UserID: 3}, ports.MarketerSearchQuery{MinPrice: &minPrice})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || repo.searchQuery.PageSize != 20 || repo.searchQuery.MinPrice == nil {
		t.Fatalf("unexpected search query: %+v", repo.searchQuery)
	}

	service = NewMarketerService(repo, &fakeCatalogRepository{}, &fakeMembershipRepository{})
	if _, err := service.Search(context.Background(), Actor{UserID: 3}, ports.MarketerSearchQuery{}); !errors.Is(err, domain.ErrCreatorRequired) {
		t.Fatalf("expected creator requirement, got %v", err)
	}
}

func TestMarketerServiceSearchAcceptsSupportedSorts(t *testing.T) {
	sorts := []string{"price_asc", "price_desc", "rating_asc", "rating_desc"}
	for _, sort := range sorts {
		t.Run(sort, func(t *testing.T) {
			repo := &fakeMarketerProfileRepository{}
			service := NewMarketerService(repo, &fakeCatalogRepository{}, &fakeMembershipRepository{creator: true})

			if _, err := service.Search(context.Background(), Actor{UserID: 3}, ports.MarketerSearchQuery{Sort: sort}); err != nil {
				t.Fatalf("expected sort %q to be accepted, got %v", sort, err)
			}
			if repo.searchQuery.Sort != sort {
				t.Fatalf("expected sort %q to reach repository, got %q", sort, repo.searchQuery.Sort)
			}
		})
	}
}

func TestMarketerServiceSearchRejectsUnsupportedSort(t *testing.T) {
	repo := &fakeMarketerProfileRepository{}
	service := NewMarketerService(repo, &fakeCatalogRepository{}, &fakeMembershipRepository{creator: true})

	if _, err := service.Search(context.Background(), Actor{UserID: 3}, ports.MarketerSearchQuery{Sort: "price_random"}); !errors.Is(err, domain.ErrInvalidMarketerProfile) {
		t.Fatalf("expected unsupported sort to be rejected, got %v", err)
	}
	if repo.searchQuery.Sort != "" {
		t.Fatalf("expected repository not to be called, got query %+v", repo.searchQuery)
	}
}
