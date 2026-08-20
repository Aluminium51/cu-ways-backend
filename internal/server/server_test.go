package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type checker struct{}

func (checker) Ping(context.Context) error {
	return nil
}

func TestNewAddsRequestID(t *testing.T) {
	app := New(Dependencies{
		Logger:           zerolog.Nop(),
		ReadinessChecker: checker{},
		ReadinessTimeout: time.Second,
	})

	res, err := app.Test(httptest.NewRequest("GET", "/healthz", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID response header")
	}
}
