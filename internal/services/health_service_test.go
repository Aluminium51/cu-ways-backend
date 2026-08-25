package services

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeChecker struct {
	err error
}

func (f fakeChecker) Ping(context.Context) error {
	return f.err
}

func TestHealthServiceCheckReturnsDatabaseError(t *testing.T) {
	want := errors.New("database offline")
	service := NewHealthService(fakeChecker{err: want}, time.Second)

	if err := service.Check(context.Background()); !errors.Is(err, want) {
		t.Fatalf("expected database error, got %v", err)
	}
}

func TestHealthServiceCheckHonorsTimeout(t *testing.T) {
	service := NewHealthService(blockingChecker{}, time.Millisecond)

	if err := service.Check(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

type blockingChecker struct{}

func (blockingChecker) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
