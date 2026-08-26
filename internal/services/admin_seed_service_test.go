package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

func TestAdminSeedServiceCreatesAdminWithHashedPassword(t *testing.T) {
	repo := newFakeUserRepository()
	hasher := &fakePasswordHasher{hashValue: "generated-hash"}
	service := NewAdminSeedService(repo, hasher)

	result, err := service.Seed(context.Background(), SeedAdminInput{
		Name:     " CU Ways Admin ",
		Email:    " ADMIN@EXAMPLE.COM ",
		Password: "local-admin-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.User.Name != "CU Ways Admin" || result.User.Email != "admin@example.com" {
		t.Fatalf("unexpected seed result: %+v", result)
	}
	if result.User.Role != domain.RoleAdmin || result.User.PasswordHash == nil || *result.User.PasswordHash != "generated-hash" {
		t.Fatalf("expected admin role and hash, got %+v", result.User)
	}
	if hasher.lastInput != "local-admin-password" {
		t.Fatal("expected seed password to be passed to the hasher")
	}
}

func TestAdminSeedServicePromotesWithoutChangingPassword(t *testing.T) {
	oldHash := "existing-hash"
	repo := newFakeUserRepository(&domain.User{
		UserID:       4,
		Name:         "Existing User",
		Email:        "admin@example.com",
		PasswordHash: &oldHash,
		Role:         domain.RoleUser,
	})
	hasher := &fakePasswordHasher{hashValue: "must-not-be-used"}
	service := NewAdminSeedService(repo, hasher)

	result, err := service.Seed(context.Background(), SeedAdminInput{
		Name:     "Ignored Name",
		Email:    "ADMIN@example.com",
		Password: "seed-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created || result.User.Role != domain.RoleAdmin || result.User.PasswordHash == nil || *result.User.PasswordHash != oldHash {
		t.Fatalf("expected promoted account with original password, got %+v", result)
	}
	if hasher.lastInput != "" {
		t.Fatal("promoting an existing account must not hash or replace its password")
	}
}

func TestAdminSeedServiceIsIdempotent(t *testing.T) {
	repo := newFakeUserRepository()
	service := NewAdminSeedService(repo, &fakePasswordHasher{hashValue: "first-hash"})
	input := SeedAdminInput{Name: "Admin", Email: "admin@example.com", Password: "seed-password"}

	first, err := service.Seed(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	firstHash := *first.User.PasswordHash
	second, err := service.Seed(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || len(repo.users) != 1 || *second.User.PasswordHash != firstHash || second.User.Role != domain.RoleAdmin {
		t.Fatalf("expected idempotent seed, first=%+v second=%+v users=%d", first, second, len(repo.users))
	}
}

func TestAdminSeedServiceRejectsDeletedAndPasswordlessUsers(t *testing.T) {
	deletedAt := time.Now()
	hash := "existing-hash"
	repo := newFakeUserRepository(
		&domain.User{UserID: 1, Email: "deleted@example.com", PasswordHash: &hash, DeletedAt: &deletedAt},
		&domain.User{UserID: 2, Email: "passwordless@example.com"},
	)
	service := NewAdminSeedService(repo, &fakePasswordHasher{})
	input := SeedAdminInput{Name: "Admin", Password: "seed-password"}

	input.Email = "deleted@example.com"
	if _, err := service.Seed(context.Background(), input); !errors.Is(err, ErrAdminSeedUserDeleted) {
		t.Fatalf("expected deleted account error, got %v", err)
	}
	input.Email = "passwordless@example.com"
	if _, err := service.Seed(context.Background(), input); !errors.Is(err, ErrAdminSeedPasswordless) {
		t.Fatalf("expected passwordless account error, got %v", err)
	}
}

func TestAdminSeedServicePropagatesRepositoryError(t *testing.T) {
	repo := newFakeUserRepository()
	want := errors.New("database unavailable")
	repo.findByEmailErr = want
	service := NewAdminSeedService(repo, &fakePasswordHasher{})

	_, err := service.Seed(context.Background(), SeedAdminInput{
		Name:     "Admin",
		Email:    "admin@example.com",
		Password: "seed-password",
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected repository error, got %v", err)
	}
}

func TestAdminSeedServiceHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewAdminSeedService(newFakeUserRepository(), &fakePasswordHasher{})

	_, err := service.Seed(ctx, SeedAdminInput{
		Name:     "Admin",
		Email:    "admin@example.com",
		Password: "seed-password",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
