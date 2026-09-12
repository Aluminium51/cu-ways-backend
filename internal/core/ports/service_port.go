package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type ServiceRepository interface {
	ListByMarketer(ctx context.Context, userID int32) ([]domain.Service, error)
	Create(ctx context.Context, service *domain.Service) error
	Update(ctx context.Context, userID, serviceID int32, patch ServicePatch) (*domain.Service, error)
	Delete(ctx context.Context, userID, serviceID int32) error
}

type ServicePatch struct {
	ServiceTypeSet bool
	ServiceType    string
	ScopeTextSet   bool
	ScopeText      *string
	PriceSet       bool
	Price          string
}
