package postgres

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

// Integration fixtures roll back rows but consume sequences, so they must use
// an explicitly selected disposable database rather than normal app settings.
func validateIntegrationDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return errors.New("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return errors.New("TEST_DATABASE_URL must use localhost or a loopback IP")
	}
	database := strings.TrimPrefix(u.Path, "/")
	if !strings.HasSuffix(database, "_test") || strings.Contains(database, "/") {
		return errors.New("TEST_DATABASE_URL must select a disposable database whose name ends with _test")
	}
	parameters, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("TEST_DATABASE_URL contains invalid connection parameters")
	}
	for name := range parameters {
		// pgx permits query parameters to override the URL's host/database.
		// Restrict options so the checked destination is the actual destination.
		if name != "sslmode" && name != "connect_timeout" {
			return errors.New("TEST_DATABASE_URL supports only sslmode and connect_timeout query parameters")
		}
	}
	return nil
}

func TestIntegrationDatabaseURLGuard(t *testing.T) {
	checks := []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "localhost", url: "postgresql://test_user:test_password@localhost:5432/cuways_test?sslmode=disable", ok: true},
		{name: "loopback", url: "postgres://127.0.0.1:5432/cuways_test", ok: true},
		{name: "ipv6_loopback", url: "postgres://[::1]:5432/cuways_test", ok: true},
		{name: "app_database", url: "postgres://localhost/cuway_database"},
		{name: "public_host", url: "postgres://example.com/cuways_test"},
		{name: "private_host", url: "postgres://10.0.0.1/cuways_test"},
		{name: "missing_database", url: "postgres://localhost"},
		{name: "wrong_scheme", url: "https://localhost/cuways_test"},
		{name: "invalid_url", url: "postgres://fixture_password%xx@localhost/cuways_test"},
		{name: "nested_path", url: "postgres://localhost/app/cuways_test"},
		{name: "host_override", url: "postgres://localhost/cuways_test?host=example.com"},
		{name: "database_override", url: "postgres://localhost/cuways_test?dbname=app_database"},
		{name: "service_override", url: "postgres://localhost/cuways_test?service=shared_database"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := validateIntegrationDatabaseURL(check.url)
			if (err == nil) != check.ok {
				t.Fatalf("accepted=%t, expected=%t", err == nil, check.ok)
			}
			if err != nil && strings.Contains(err.Error(), "fixture_password") {
				t.Fatal("validation error exposed credentials")
			}
		})
	}
}
