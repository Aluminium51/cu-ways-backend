package health

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type Service struct {
	checker ports.ReadinessChecker
	timeout time.Duration
}

func NewService(checker ports.ReadinessChecker, timeout time.Duration) *Service {
	return &Service{checker: checker, timeout: timeout}
}

func (s *Service) Check(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.checker.Ping(checkCtx)
}
