package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type mockDataFakeHasher struct {
	hashValue string
	lastInput string
	err       error
}

func (f *mockDataFakeHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.lastInput = password
	if f.err != nil {
		return "", f.err
	}
	return f.hashValue, nil
}

func (f *mockDataFakeHasher) Compare(context.Context, string, string) error { return nil }

type mockDataFakeRepository struct {
	seed ports.MockDataSeed
	err  error
}

func (f *mockDataFakeRepository) SeedMockData(_ context.Context, seed ports.MockDataSeed) error {
	f.seed = seed
	return f.err
}

func TestMockDataSeederBuildsTwentyUsersWithExpectedMemberships(t *testing.T) {
	repo := &mockDataFakeRepository{}
	hasher := &mockDataFakeHasher{hashValue: "argon2id-hash"}
	seeder := NewMockDataSeeder(repo, hasher)

	summary, err := seeder.Seed(context.Background(), "mock-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Users != 20 || summary.Creators != 14 || summary.Marketers != 14 || summary.Both != 8 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(repo.seed.Users) != 20 || len(repo.seed.Jobs) != 11 {
		t.Fatalf("unexpected seed sizes: users=%d jobs=%d", len(repo.seed.Users), len(repo.seed.Jobs))
	}
	if hasher.lastInput != "mock-password-123" {
		t.Fatalf("expected password to be passed to hasher, got %q", hasher.lastInput)
	}

	creatorOnly, marketerOnly, both := 0, 0, 0
	for _, user := range repo.seed.Users {
		if user.PasswordHash != "argon2id-hash" {
			t.Fatalf("expected generated password hash, got %q", user.PasswordHash)
		}
		if user.Creator && user.Marketer != nil {
			both++
		} else if user.Creator {
			creatorOnly++
		} else if user.Marketer != nil {
			marketerOnly++
		}
	}
	if creatorOnly != 6 || marketerOnly != 6 || both != 8 {
		t.Fatalf("unexpected memberships: creator-only=%d marketer-only=%d both=%d", creatorOnly, marketerOnly, both)
	}
}

func TestMockDataSeederRejectsInvalidPasswordAndCanceledContext(t *testing.T) {
	repo := &mockDataFakeRepository{}
	hasher := &mockDataFakeHasher{hashValue: "must-not-be-used"}
	seeder := NewMockDataSeeder(repo, hasher)

	if _, err := seeder.Seed(context.Background(), "short"); !errors.Is(err, domain.ErrInvalidUser) {
		t.Fatalf("expected invalid password error, got %v", err)
	}
	if len(repo.seed.Users) != 0 {
		t.Fatal("invalid password must not seed data")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := seeder.Seed(ctx, "valid-mock-password"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestBuildMockDataSeedUsesProvidedTimestamp(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	seed, _ := BuildMockDataSeed(now, "hash")
	if len(seed.Users) == 0 || !seed.Users[0].CreatedAt.Before(now) {
		t.Fatalf("expected user timestamps before seed time, got %+v", seed.Users)
	}
	if len(seed.Jobs) == 0 || !seed.Jobs[0].CreatedAt.Before(now) {
		t.Fatalf("expected job timestamps before seed time, got %+v", seed.Jobs)
	}
}
