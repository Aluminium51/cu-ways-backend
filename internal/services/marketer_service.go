package services

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

const (
	DefaultMarketerPage     = 1
	DefaultMarketerPageSize = 20
	MaxMarketerPageSize     = 100
	MaxExperienceYears      = 80
	MaxProfileTextLength    = 5000
)

type MarketerProfileInput struct {
	Bio                string
	ExperienceYears    int32
	AvailabilityStatus string
	AvailabilityText   string
	ExpertiseSlugs     []string
	CampusSlugs        []string
}

type MarketerService struct {
	profiles    ports.MarketerProfileRepository
	catalog     ports.CatalogRepository
	memberships ports.MembershipRepository
}

func NewMarketerService(profiles ports.MarketerProfileRepository, catalog ports.CatalogRepository, memberships ports.MembershipRepository) *MarketerService {
	return &MarketerService{profiles: profiles, catalog: catalog, memberships: memberships}
}

func (s *MarketerService) GetProfile(ctx context.Context, actor Actor) (*domain.Marketer, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrUserForbidden
	}
	return s.profiles.FindProfile(ctx, actor.UserID)
}

func (s *MarketerService) GetDetail(ctx context.Context, actor Actor, marketerID int32) (*domain.MarketerDetail, error) {
	if actor.UserID < 1 || marketerID < 1 {
		return nil, domain.ErrMarketerProfileNotFound
	}
	if !actor.IsAdmin {
		isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
		if err != nil {
			return nil, err
		}
		if !isCreator {
			return nil, domain.ErrCreatorRequired
		}
	}
	return s.profiles.FindDetail(ctx, marketerID)
}

func (s *MarketerService) SaveProfile(ctx context.Context, actor Actor, input MarketerProfileInput) (*domain.Marketer, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrUserForbidden
	}

	profile, err := normalizeMarketerProfile(input)
	if err != nil {
		return nil, err
	}

	expertise, err := s.catalog.FindExpertiseBySlugs(ctx, profile.expertiseSlugs)
	if err != nil {
		return nil, err
	}
	if len(expertise) != len(profile.expertiseSlugs) {
		return nil, domain.ErrCatalogOptionNotFound
	}
	campuses, err := s.catalog.FindCampusesBySlugs(ctx, profile.campusSlugs)
	if err != nil {
		return nil, err
	}
	if len(campuses) != len(profile.campusSlugs) {
		return nil, domain.ErrCatalogOptionNotFound
	}

	entity := &domain.Marketer{
		UserID:             actor.UserID,
		Bio:                profile.bio,
		ExperienceYears:    profile.experienceYears,
		AvailabilityStatus: profile.availabilityStatus,
		AvailabilityText:   profile.availabilityText,
		Expertise:          expertise,
		Campuses:           campuses,
	}
	expertiseIDs := make([]int32, 0, len(expertise))
	for _, option := range expertise {
		expertiseIDs = append(expertiseIDs, option.ExpertiseID)
	}
	campusIDs := make([]int32, 0, len(campuses))
	for _, option := range campuses {
		campusIDs = append(campusIDs, option.CampusID)
	}

	if err := s.profiles.SaveProfile(ctx, entity, expertiseIDs, campusIDs); err != nil {
		return nil, err
	}
	return s.profiles.FindProfile(ctx, actor.UserID)
}

func (s *MarketerService) Search(ctx context.Context, actor Actor, query ports.MarketerSearchQuery) (ports.MarketerPage, error) {
	if !actor.IsAdmin {
		isCreator, err := s.memberships.IsCreator(ctx, actor.UserID)
		if err != nil {
			return ports.MarketerPage{}, err
		}
		if !isCreator {
			return ports.MarketerPage{}, domain.ErrCreatorRequired
		}
	}
	if query.Page == 0 {
		query.Page = DefaultMarketerPage
	}
	if query.PageSize == 0 {
		query.PageSize = DefaultMarketerPageSize
	}
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > MaxMarketerPageSize {
		return ports.MarketerPage{}, domain.ErrInvalidMarketerProfile
	}
	if query.MinPrice != nil && query.MinPrice.IsNegative() {
		return ports.MarketerPage{}, domain.ErrInvalidPrice
	}
	if query.MaxPrice != nil && query.MaxPrice.IsNegative() {
		return ports.MarketerPage{}, domain.ErrInvalidPrice
	}
	if query.MinPrice != nil && query.MaxPrice != nil && query.MinPrice.GreaterThan(*query.MaxPrice) {
		return ports.MarketerPage{}, domain.ErrInvalidPrice
	}
	if query.MinRating != nil && (math.IsNaN(*query.MinRating) || math.IsInf(*query.MinRating, 0) || *query.MinRating < 1 || *query.MinRating > 5) {
		return ports.MarketerPage{}, domain.ErrInvalidMarketerProfile
	}
	if query.MinExperienceYears != nil && (*query.MinExperienceYears < 0 || *query.MinExperienceYears > MaxExperienceYears) {
		return ports.MarketerPage{}, domain.ErrInvalidMarketerProfile
	}
	if query.AvailabilityStatus != nil && !validAvailability(*query.AvailabilityStatus) {
		return ports.MarketerPage{}, domain.ErrInvalidAvailability
	}
	if query.Sort != "" && query.Sort != "price_asc" && query.Sort != "rating_desc" {
		return ports.MarketerPage{}, domain.ErrInvalidMarketerProfile
	}

	query.ExpertiseSlugs = normalizeSlugs(query.ExpertiseSlugs)
	query.CampusSlugs = normalizeSlugs(query.CampusSlugs)
	return s.profiles.Search(ctx, query)
}

type normalizedMarketerProfile struct {
	bio                string
	experienceYears    int32
	availabilityStatus domain.AvailabilityStatus
	availabilityText   string
	expertiseSlugs     []string
	campusSlugs        []string
}

func normalizeMarketerProfile(input MarketerProfileInput) (normalizedMarketerProfile, error) {
	bio := strings.TrimSpace(input.Bio)
	availabilityText := strings.TrimSpace(input.AvailabilityText)
	if bio == "" || availabilityText == "" || utf8.RuneCountInString(bio) > MaxProfileTextLength || utf8.RuneCountInString(availabilityText) > MaxProfileTextLength {
		return normalizedMarketerProfile{}, domain.ErrInvalidMarketerProfile
	}
	if input.ExperienceYears < 0 || input.ExperienceYears > MaxExperienceYears {
		return normalizedMarketerProfile{}, domain.ErrInvalidMarketerProfile
	}
	availability := domain.AvailabilityStatus(strings.ToLower(strings.TrimSpace(input.AvailabilityStatus)))
	if !validAvailability(availability) {
		return normalizedMarketerProfile{}, domain.ErrInvalidAvailability
	}
	return normalizedMarketerProfile{
		bio:                bio,
		experienceYears:    input.ExperienceYears,
		availabilityStatus: availability,
		availabilityText:   availabilityText,
		expertiseSlugs:     normalizeSlugs(input.ExpertiseSlugs),
		campusSlugs:        normalizeSlugs(input.CampusSlugs),
	}, nil
}

func validAvailability(status domain.AvailabilityStatus) bool {
	return status == domain.AvailabilityAvailable || status == domain.AvailabilityLimited || status == domain.AvailabilityUnavailable
}

func normalizeSlugs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
