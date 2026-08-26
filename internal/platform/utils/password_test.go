package utils

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestArgon2idPasswordHasherRoundTrip(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()
	password := "correct horse battery staple"

	first, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected independent password hashes to use different salts")
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$") {
		t.Fatalf("unexpected password hash format: %s", first)
	}
	if err := hasher.Compare(context.Background(), first, password); err != nil {
		t.Fatalf("expected password to verify: %v", err)
	}
	if !errors.Is(hasher.Compare(context.Background(), first, "wrong password"), ErrPasswordMismatch) {
		t.Fatal("expected wrong password to be rejected")
	}
}

func TestArgon2idPasswordHasherRejectsMalformedHash(t *testing.T) {
	hasher := NewArgon2idPasswordHasher()
	if !errors.Is(hasher.Compare(context.Background(), "not-a-hash", "password"), ErrInvalidPasswordHash) {
		t.Fatal("expected malformed hash to be rejected")
	}
}

func TestArgon2idPasswordHasherHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hasher := NewArgon2idPasswordHasher()

	if _, err := hasher.Hash(ctx, "password"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled hash context, got %v", err)
	}
	if err := hasher.Compare(ctx, "not-a-hash", "password"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled compare context, got %v", err)
	}
}
