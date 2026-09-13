package ports

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// MockDataSeed contains deterministic development/test fixtures. It is kept
// separate from migrations so the fixtures can be recreated without changing
// the database schema.
type MockDataSeed struct {
	Users []MockUserSeed
	Jobs  []MockJobSeed
}

type MockUserSeed struct {
	Name         string
	Email        string
	Phone        *string
	LineID       *string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	Creator      bool
	Marketer     *MockMarketerSeed
}

type MockMarketerSeed struct {
	Bio                string
	ExperienceYears    int32
	AvailabilityStatus string
	AvailabilityText   string
	ExpertiseSlugs     []string
	CampusSlugs        []string
	Services           []MockServiceSeed
}

type MockServiceSeed struct {
	ServiceType string
	ScopeText   *string
	Price       decimal.Decimal
	CreatedAt   time.Time
	DeletedAt   *time.Time
}

type MockJobSeed struct {
	CreatorEmail  string
	MarketerEmail string
	OfferedPrice  decimal.Decimal
	Rating        *int32
	CreatedAt     time.Time
}

type MockDataRepository interface {
	SeedMockData(ctx context.Context, seed MockDataSeed) error
}
