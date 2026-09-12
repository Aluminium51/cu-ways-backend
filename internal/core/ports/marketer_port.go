package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/shopspring/decimal"
)

type MarketerProfileRepository interface {
	FindProfile(ctx context.Context, userID int32) (*domain.Marketer, error)
	FindDetail(ctx context.Context, userID int32) (*domain.MarketerDetail, error)
	SaveProfile(ctx context.Context, profile *domain.Marketer, expertiseIDs, campusIDs []int32) error
	Search(ctx context.Context, query MarketerSearchQuery) (MarketerPage, error)
}

type MarketerSearchQuery struct {
	Page               int
	PageSize           int
	MinPrice           *decimal.Decimal
	MaxPrice           *decimal.Decimal
	ExpertiseSlugs     []string
	CampusSlugs        []string
	MinRating          *float64
	MinExperienceYears *int32
	AvailabilityStatus *domain.AvailabilityStatus
	Sort               string
}

type MarketerPage struct {
	Items    []domain.MarketerSearchResult
	Page     int
	PageSize int
	Total    int64
}
