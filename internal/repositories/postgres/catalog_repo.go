package postgres

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"gorm.io/gorm"
)

type CatalogRepository struct {
	db *gorm.DB
}

var _ ports.CatalogRepository = (*CatalogRepository)(nil)

func NewCatalogRepository(db *gorm.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) FindExpertiseBySlugs(ctx context.Context, slugs []string) ([]domain.Expertise, error) {
	options := make([]domain.Expertise, 0, len(slugs))
	if len(slugs) == 0 {
		return options, nil
	}
	if err := r.db.WithContext(ctx).Where("slug IN ?", slugs).Order("expertise_id ASC").Find(&options).Error; err != nil {
		return nil, err
	}
	return options, nil
}

func (r *CatalogRepository) FindCampusesBySlugs(ctx context.Context, slugs []string) ([]domain.Campus, error) {
	options := make([]domain.Campus, 0, len(slugs))
	if len(slugs) == 0 {
		return options, nil
	}
	if err := r.db.WithContext(ctx).Where("slug IN ?", slugs).Order("campus_id ASC").Find(&options).Error; err != nil {
		return nil, err
	}
	return options, nil
}
