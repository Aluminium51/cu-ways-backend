package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// CatalogRepository provides the curated values used by marketer profiles.
// Services deal in domain values and do not know how those values are stored.
type CatalogRepository interface {
	FindExpertiseBySlugs(ctx context.Context, slugs []string) ([]domain.Expertise, error)
	FindCampusesBySlugs(ctx context.Context, slugs []string) ([]domain.Campus, error)
}
