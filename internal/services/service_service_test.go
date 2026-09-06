package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeServiceRepository struct {
	created *domain.Service
}

func (f *fakeServiceRepository) ListByMarketer(context.Context, int32) ([]domain.Service, error) {
	return nil, nil
}

func (f *fakeServiceRepository) Create(_ context.Context, service *domain.Service) error {
	f.created = service
	return nil
}

func (f *fakeServiceRepository) Update(context.Context, int32, int32, ports.ServicePatch) (*domain.Service, error) {
	return nil, nil
}

func (f *fakeServiceRepository) Delete(context.Context, int32, int32) error { return nil }

func TestServiceServiceCreatesNormalizedServiceForMarketer(t *testing.T) {
	repo := &fakeServiceRepository{}
	service := NewServiceService(repo, &fakeMembershipRepository{marketer: true})
	result, err := service.Create(context.Background(), Actor{UserID: 8}, CreateServiceInput{
		ServiceType: "  Data collection ", ScopeText: stringPointer("  On campus "), Price: "1250.50",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ServiceType != "Data collection" || result.Price.StringFixed(2) != "1250.50" || result.CreatedAt.IsZero() {
		t.Fatalf("unexpected service: %+v", result)
	}
}

func TestServiceServiceRejectsNonMarketer(t *testing.T) {
	service := NewServiceService(&fakeServiceRepository{}, &fakeMembershipRepository{})
	_, err := service.Create(context.Background(), Actor{UserID: 1}, CreateServiceInput{ServiceType: "Survey", Price: "10"})
	if !errors.Is(err, domain.ErrMarketerRequired) {
		t.Fatalf("expected marketer requirement, got %v", err)
	}
}

func stringPointer(value string) *string { return &value }
