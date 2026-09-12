package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
)

const MockUserCount = 20

type MockDataSummary struct {
	Users     int
	Creators  int
	Marketers int
	Both      int
}

type MockDataSeeder struct {
	repo   ports.MockDataRepository
	hasher ports.PasswordHasher
	now    func() time.Time
}

func NewMockDataSeeder(repo ports.MockDataRepository, hasher ports.PasswordHasher) *MockDataSeeder {
	return &MockDataSeeder{repo: repo, hasher: hasher, now: time.Now}
}

func (s *MockDataSeeder) Seed(ctx context.Context, password string) (MockDataSummary, error) {
	if err := ctx.Err(); err != nil {
		return MockDataSummary{}, err
	}
	if err := validatePassword(password); err != nil {
		return MockDataSummary{}, err
	}
	passwordHash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return MockDataSummary{}, err
	}
	seed, summary := BuildMockDataSeed(s.now().UTC(), passwordHash)
	if err := s.repo.SeedMockData(ctx, seed); err != nil {
		return MockDataSummary{}, err
	}
	return summary, nil
}

func BuildMockDataSeed(now time.Time, passwordHash string) (ports.MockDataSeed, MockDataSummary) {
	const creatorOnlyCount = 6
	const marketerOnlyCount = 6
	const bothCount = 8

	users := make([]ports.MockUserSeed, 0, MockUserCount)
	marketerIndex := 0
	summary := MockDataSummary{
		Users:     MockUserCount,
		Creators:  creatorOnlyCount + bothCount,
		Marketers: marketerOnlyCount + bothCount,
		Both:      bothCount,
	}

	for index := 1; index <= MockUserCount; index++ {
		category, categoryIndex, isCreator, isMarketer := mockUserCategory(index, creatorOnlyCount, marketerOnlyCount)
		name := fmt.Sprintf("Mock %s %02d", mockCategoryLabel(category), categoryIndex)
		email := fmt.Sprintf("mock.%s.%02d@example.test", category, categoryIndex)
		phone := fmt.Sprintf("080000%04d", index)
		lineID := fmt.Sprintf("mock_%s_%02d", category, categoryIndex)
		user := ports.MockUserSeed{
			Name:         name,
			Email:        email,
			Phone:        &phone,
			LineID:       &lineID,
			PasswordHash: passwordHash,
			Role:         domain.RoleUser,
			CreatedAt:    now.Add(-time.Duration(index) * time.Hour),
			Creator:      isCreator,
		}
		if isMarketer {
			marketerIndex++
			user.Marketer = buildMockMarketer(marketerIndex, now, category, categoryIndex)
		}
		users = append(users, user)
	}

	return ports.MockDataSeed{Users: users, Jobs: buildMockJobs(now)}, summary
}

func mockUserCategory(index, creatorOnlyCount, marketerOnlyCount int) (string, int, bool, bool) {
	switch {
	case index <= creatorOnlyCount:
		return "creator-only", index, true, false
	case index <= creatorOnlyCount+marketerOnlyCount:
		return "marketer-only", index - creatorOnlyCount, false, true
	default:
		return "both", index - creatorOnlyCount - marketerOnlyCount, true, true
	}
}

func mockCategoryLabel(category string) string {
	switch category {
	case "creator-only":
		return "Creator Only"
	case "marketer-only":
		return "Marketer Only"
	default:
		return "Creator Marketer"
	}
}

func buildMockMarketer(index int, now time.Time, category string, categoryIndex int) *ports.MockMarketerSeed {
	availability := []string{"available", "limited", "unavailable"}
	expertise := []string{
		"survey-distribution",
		"participant-recruitment",
		"data-collection",
		"quantitative-analysis",
		"qualitative-analysis",
		"report-preparation",
	}
	campuses := []string{"cu-main-campus", "cu-health-sciences-campus", "off-campus", "online-remote"}

	profile := &ports.MockMarketerSeed{
		Bio:                fmt.Sprintf("Experienced mock marketer %02d for search and filter testing.", index),
		ExperienceYears:    int32(index + 1),
		AvailabilityStatus: availability[(index-1)%len(availability)],
		AvailabilityText:   fmt.Sprintf("Mock availability for %s %02d", category, categoryIndex),
		ExpertiseSlugs:     []string{expertise[(index-1)%len(expertise)]},
		CampusSlugs:        []string{campuses[(index-1)%len(campuses)]},
		Services:           mockServiceSeeds(index, now),
	}
	if index%3 == 0 {
		profile.ExpertiseSlugs = append(profile.ExpertiseSlugs, expertise[index%len(expertise)])
		profile.CampusSlugs = append(profile.CampusSlugs, campuses[index%len(campuses)])
	}
	return profile
}

func mockServiceSeeds(index int, now time.Time) []ports.MockServiceSeed {
	prices := [][]string{
		{"250.00", "750.00"},
		{"500.00"},
		{"750.00"},
		{"1000.00"},
		{"1500.00"},
		{"2000.00"},
		{"300.00", "1200.00"},
		{"600.00", "1800.00"},
		{"900.00"},
		{"1200.00"},
		{"1800.00"},
		{"2500.00"},
		{"3500.00"},
		{},
	}
	if index == len(prices) {
		deletedAt := now.Add(-24 * time.Hour)
		return []ports.MockServiceSeed{{
			ServiceType: "Mock Archived Service 14",
			ScopeText:   mockStringPointer("Archived mock listing for active-service filtering."),
			Price:       decimal.RequireFromString("9000.00"),
			CreatedAt:   now.Add(-14 * 24 * time.Hour),
			DeletedAt:   &deletedAt,
		}}
	}

	result := make([]ports.MockServiceSeed, 0, len(prices[index-1]))
	for serviceIndex, rawPrice := range prices[index-1] {
		result = append(result, ports.MockServiceSeed{
			ServiceType: fmt.Sprintf("Mock Service %02d-%02d", index, serviceIndex+1),
			ScopeText:   mockStringPointer(fmt.Sprintf("Mock service scope %02d-%02d.", index, serviceIndex+1)),
			Price:       decimal.RequireFromString(rawPrice),
			CreatedAt:   now.Add(-time.Duration(index*10+serviceIndex) * time.Hour),
		})
	}
	return result
}

func buildMockJobs(now time.Time) []ports.MockJobSeed {
	ratings := []struct {
		marketerIndex int
		rating        int32
	}{
		{marketerIndex: 1, rating: 5},
		{marketerIndex: 2, rating: 4},
		{marketerIndex: 3, rating: 3},
		{marketerIndex: 4, rating: 2},
		{marketerIndex: 5, rating: 1},
		{marketerIndex: 7, rating: 5},
		{marketerIndex: 9, rating: 4},
		{marketerIndex: 10, rating: 3},
		{marketerIndex: 11, rating: 2},
		{marketerIndex: 12, rating: 1},
		{marketerIndex: 13, rating: 5},
	}
	jobs := make([]ports.MockJobSeed, 0, len(ratings))
	for _, item := range ratings {
		category, categoryIndex := mockMarketerEmailParts(item.marketerIndex)
		creatorIndex := (item.marketerIndex-1)%6 + 1
		jobs = append(jobs, ports.MockJobSeed{
			CreatorEmail:  fmt.Sprintf("mock.creator-only.%02d@example.test", creatorIndex),
			MarketerEmail: fmt.Sprintf("mock.%s.%02d@example.test", category, categoryIndex),
			OfferedPrice:  decimal.NewFromInt(int64(500 + item.marketerIndex*100)),
			Rating:        mockInt32Pointer(item.rating),
			CreatedAt:     now.Add(-time.Duration(item.marketerIndex) * 24 * time.Hour),
		})
	}
	return jobs
}

func mockMarketerEmailParts(index int) (string, int) {
	if index <= 6 {
		return "marketer-only", index
	}
	return "both", index - 6
}

func mockStringPointer(value string) *string { return &value }

func mockInt32Pointer(value int32) *int32 { return &value }
