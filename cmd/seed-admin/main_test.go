package main

import (
	"strings"
	"testing"

	"github.com/Aluminium51/cu-way-backend/internal/config"
)

func TestValidateSeedEnvironment(t *testing.T) {
	for _, environment := range []string{"development", "test"} {
		if err := validateSeedEnvironment(environment); err != nil {
			t.Fatalf("expected %s to be allowed: %v", environment, err)
		}
	}
	if err := validateSeedEnvironment("production"); err == nil {
		t.Fatal("expected production seed to be rejected")
	}
}

func TestSeedAdminInputRequiresCredentialsAndDefaultsName(t *testing.T) {
	input, err := seedAdminInput(config.SeedAdminConfig{Email: "admin@example.com", Password: "local-password"})
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != defaultAdminName || input.Email != "admin@example.com" || input.Password != "local-password" {
		t.Fatalf("unexpected seed input: %+v", input)
	}

	if _, err := seedAdminInput(config.SeedAdminConfig{Password: "local-password"}); err == nil || !strings.Contains(err.Error(), "SEED_ADMIN_EMAIL") {
		t.Fatalf("expected missing email error, got %v", err)
	}
	if _, err := seedAdminInput(config.SeedAdminConfig{Email: "admin@example.com"}); err == nil || !strings.Contains(err.Error(), "SEED_ADMIN_PASSWORD") {
		t.Fatalf("expected missing password error, got %v", err)
	}
}
