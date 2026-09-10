package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type fakeServiceRepository struct {
	created         *domain.Service
	updateUserID    int32
	updateServiceID int32
	deleteUserID    int32
	deleteServiceID int32
	updateCalls     int
}

func (f *fakeServiceRepository) ListByMarketer(context.Context, int32) ([]domain.Service, error) {
	return nil, nil
}

func (f *fakeServiceRepository) Create(_ context.Context, service *domain.Service) error {
	f.created = service
	return nil
}

func (f *fakeServiceRepository) Update(_ context.Context, userID, serviceID int32, _ ports.ServicePatch) (*domain.Service, error) {
	f.updateCalls++
	f.updateUserID = userID
	f.updateServiceID = serviceID
	return &domain.Service{ServiceID: serviceID, UserID: userID}, nil
}

func TestServiceServiceRejectsInvalidServiceTypeBeforeRepositoryUpdate(t *testing.T) {
	repo := &fakeServiceRepository{}
	service := NewServiceService(repo, &fakeMembershipRepository{marketer: true})

	_, err := service.Update(context.Background(), Actor{UserID: 8}, 21, UpdateServiceInput{
		ServiceTypeSet: true,
		ServiceType:    string(make([]byte, 101)),
	})
	if !errors.Is(err, domain.ErrInvalidService) {
		t.Fatalf("expected invalid service error, got %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("expected repository update not to run, got %d calls", repo.updateCalls)
	}
}

func (f *fakeServiceRepository) Delete(_ context.Context, userID, serviceID int32) error {
	f.deleteUserID = userID
	f.deleteServiceID = serviceID
	return nil
}

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

func TestServiceServiceScopesUpdateAndDeleteToAuthenticatedOwner(t *testing.T) {
	repo := &fakeServiceRepository{}
	service := NewServiceService(repo, &fakeMembershipRepository{marketer: true})
	actor := Actor{UserID: 8}

	if _, err := service.Update(context.Background(), actor, 21, UpdateServiceInput{
		PriceSet: true,
		Price:    "900.00",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.updateUserID != actor.UserID || repo.updateServiceID != 21 {
		t.Fatalf("update was not scoped to owner: user=%d service=%d", repo.updateUserID, repo.updateServiceID)
	}

	if err := service.Delete(context.Background(), actor, 21); err != nil {
		t.Fatal(err)
	}
	if repo.deleteUserID != actor.UserID || repo.deleteServiceID != 21 {
		t.Fatalf("delete was not scoped to owner: user=%d service=%d", repo.deleteUserID, repo.deleteServiceID)
	}
}

func stringPointer(value string) *string { return &value }
