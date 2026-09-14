package main

import "testing"

func TestValidateMockDataEnvironment(t *testing.T) {
	for _, environment := range []string{"development", "test"} {
		if err := validateMockDataEnvironment(environment); err != nil {
			t.Fatalf("expected %s to be allowed, got %v", environment, err)
		}
	}
	if err := validateMockDataEnvironment("production"); err == nil {
		t.Fatal("expected production mock data seeding to be rejected")
	}
}
