package services

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type HealthService struct {
	checker ports.ReadinessChecker
	timeout time.Duration
}

func NewHealthService(checker ports.ReadinessChecker, timeout time.Duration) *HealthService {
	return &HealthService{checker: checker, timeout: timeout}
}

func (s *HealthService) Check(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.checker.Ping(checkCtx)
}
