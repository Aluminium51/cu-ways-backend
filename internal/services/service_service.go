package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/shopspring/decimal"
)

const (
	MaxServiceTypeLength = 100
	MaxScopeTextLength   = 5000
)

type CreateServiceInput struct {
	ServiceType string
	ScopeText   *string
	Price       string
}

type UpdateServiceInput struct {
	ServiceTypeSet bool
	ServiceType    string
	ScopeTextSet   bool
	ScopeText      *string
	PriceSet       bool
	Price          string
}

type ServiceService struct {
	repo        ports.ServiceRepository
	memberships ports.MembershipRepository
}

func NewServiceService(repo ports.ServiceRepository, memberships ports.MembershipRepository) *ServiceService {
	return &ServiceService{repo: repo, memberships: memberships}
}

func (s *ServiceService) List(ctx context.Context, actor Actor) ([]domain.Service, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.ListByMarketer(ctx, actor.UserID)
}

func (s *ServiceService) Create(ctx context.Context, actor Actor, input CreateServiceInput) (*domain.Service, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	serviceType, err := normalizeServiceType(input.ServiceType)
	if err != nil {
		return nil, err
	}
	scope, err := normalizeOptionalText(input.ScopeText, MaxScopeTextLength)
	if err != nil {
		return nil, err
	}
	price, err := parsePrice(input.Price)
	if err != nil {
		return nil, err
	}
	service := &domain.Service{
		UserID:      actor.UserID,
		ServiceType: serviceType,
		ScopeText:   scope,
		Price:       price,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, service); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *ServiceService) Update(ctx context.Context, actor Actor, serviceID int32, input UpdateServiceInput) (*domain.Service, error) {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return nil, err
	}
	if !input.ServiceTypeSet && !input.ScopeTextSet && !input.PriceSet {
		return nil, domain.ErrNoServiceChanges
	}
	patch := ports.ServicePatch{
		ServiceTypeSet: input.ServiceTypeSet,
		ScopeTextSet:   input.ScopeTextSet,
		ScopeText:      input.ScopeText,
		PriceSet:       input.PriceSet,
		Price:          input.Price,
	}
	if input.ServiceTypeSet {
		serviceType, err := normalizeServiceType(input.ServiceType)
		if err != nil {
			return nil, err
		}
		patch.ServiceType = serviceType
	}
	if input.ScopeTextSet {
		var err error
		patch.ScopeText, err = normalizeOptionalText(input.ScopeText, MaxScopeTextLength)
		if err != nil {
			return nil, err
		}
	}
	if input.PriceSet {
		price, err := parsePrice(input.Price)
		if err != nil {
			return nil, err
		}
		patch.Price = price.StringFixed(2)
	}
	return s.repo.Update(ctx, actor.UserID, serviceID, patch)
}

func (s *ServiceService) Delete(ctx context.Context, actor Actor, serviceID int32) error {
	if err := s.requireMarketer(ctx, actor); err != nil {
		return err
	}
	return s.repo.Delete(ctx, actor.UserID, serviceID)
}

func (s *ServiceService) requireMarketer(ctx context.Context, actor Actor) error {
	if actor.UserID < 1 {
		return domain.ErrServiceForbidden
	}
	isMarketer, err := s.memberships.IsMarketer(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isMarketer {
		return domain.ErrMarketerRequired
	}
	return nil
}

func normalizeServiceType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > MaxServiceTypeLength {
		return "", domain.ErrInvalidService
	}
	return value, nil
}

func normalizeOptionalText(value *string, maxLength int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if utf8.RuneCountInString(normalized) > maxLength {
		return nil, domain.ErrInvalidService
	}
	return &normalized, nil
}

func parsePrice(value string) (decimal.Decimal, error) {
	price, err := decimal.NewFromString(strings.TrimSpace(value))
	maxPrice, _ := decimal.NewFromString("99999999.99")
	if err != nil || price.IsNegative() || price.Exponent() < -2 || price.GreaterThan(maxPrice) {
		return decimal.Zero, errors.Join(domain.ErrInvalidPrice, err)
	}
	return price, nil
}
