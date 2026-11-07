package main

import (
	"strings"
	"testing"
)

func TestRunRequiresDatabaseURL(t *testing.T) {
	err := run("", migrationSource)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected missing database URL error, got %v", err)
	}
}

func TestRunDoesNotExposeDatabaseCredentials(t *testing.T) {
	const secret = "private-password"
	err := run("postgres://user:"+secret+"@127.0.0.1:5432/example", "file://missing-migrations")
	if err == nil {
		t.Fatal("expected missing migrations error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("migration error exposed database credentials")
	}
}
