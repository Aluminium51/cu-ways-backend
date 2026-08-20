package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileUsesEnvironmentOverFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("APP_ENV=development\nPORT=9000\nDATABASE_URL=postgresql://file-host:5432/cuways\nSECRET_KEY=file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PORT", "9100")
	t.Setenv("DATABASE_URL", "postgresql://env-host:5432/cuways")
	t.Setenv("SECRET_KEY", "env-secret")

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9100 {
		t.Fatalf("expected environment port 9100, got %d", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgresql://env-host:5432/cuways" {
		t.Fatalf("expected environment database URL, got %q", cfg.Database.URL)
	}
}

func TestLoadFromFileAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DATABASE_URL=postgresql://localhost:5432/cuways\nSECRET_KEY=local-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"APP_ENV", "PORT", "SHUTDOWN_TIMEOUT", "READINESS_TIMEOUT", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME", "DATABASE_URL", "SECRET_KEY"} {
		t.Setenv(key, "")
		t.Cleanup(func() { os.Unsetenv(key) })
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8081 || cfg.App.Environment != "development" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadFromFileRequiresDatabaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("SECRET_KEY=local-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil || err.Error() != "DATABASE_URL is required" {
		t.Fatalf("expected database URL error, got %v", err)
	}
}

func TestLoadFromFileRequiresStrongProductionSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("APP_ENV=production\nDATABASE_URL=postgresql://localhost:5432/cuways\nSECRET_KEY=short\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil || err.Error() != "SECRET_KEY must be at least 32 characters in production" {
		t.Fatalf("expected production secret error, got %v", err)
	}
}

func TestValidateDatabaseURL(t *testing.T) {
	for _, value := range []string{"", "http://localhost/db", "postgresql://"} {
		if err := validateDatabaseURL(value); err == nil {
			t.Fatalf("expected invalid URL error for %q", value)
		}
	}
	if err := validateDatabaseURL("postgres://localhost:5432/cuways"); err != nil {
		t.Fatal(err)
	}
}
