package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ServiceRepository struct {
	db *gorm.DB
}

var _ ports.ServiceRepository = (*ServiceRepository)(nil)

func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) ListByMarketer(ctx context.Context, userID int32) ([]domain.Service, error) {
	services := make([]domain.Service, 0)
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("service_id ASC").Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (r *ServiceRepository) Create(ctx context.Context, service *domain.Service) error {
	if service.CreatedAt.IsZero() {
		service.CreatedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).Create(service).Error; err != nil {
		return err
	}
	return nil
}

func (r *ServiceRepository) Update(ctx context.Context, userID, serviceID int32, patch ports.ServicePatch) (*domain.Service, error) {
	updates := make(map[string]any)
	if patch.ServiceTypeSet {
		updates["service_type"] = patch.ServiceType
	}
	if patch.ScopeTextSet {
		updates["scope_text"] = patch.ScopeText
	}
	if patch.PriceSet {
		price, err := decimal.NewFromString(patch.Price)
		if err != nil {
			return nil, domain.ErrInvalidPrice
		}
		updates["price"] = price
	}
	if len(updates) == 0 {
		return nil, domain.ErrNoServiceChanges
	}
	result := r.db.WithContext(ctx).Model(&domain.Service{}).
		Where("service_id = ? AND user_id = ?", serviceID, userID).Updates(updates)
	if err := result.Error; err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrServiceNotFound
	}
	var service domain.Service
	if err := r.db.WithContext(ctx).Where("service_id = ? AND user_id = ?", serviceID, userID).First(&service).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrServiceNotFound
		}
		return nil, err
	}
	return &service, nil
}

func (r *ServiceRepository) Delete(ctx context.Context, userID, serviceID int32) error {
	result := r.db.WithContext(ctx).Where("service_id = ? AND user_id = ?", serviceID, userID).Delete(&domain.Service{})
	if err := result.Error; err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return domain.ErrServiceNotFound
	}
	return nil
}
