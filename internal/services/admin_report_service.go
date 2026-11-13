package services

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

var ErrInvalidReportQuery = errors.New("invalid admin report query")

type AdminReportService struct {
	repo ports.AdminReportRepository
}

func NewAdminReportService(repo ports.AdminReportRepository) *AdminReportService {
	return &AdminReportService{repo: repo}
}

func (s *AdminReportService) ListJobs(ctx context.Context, actor Actor, query ports.AdminReportQuery) (ports.AdminReportPage[ports.AdminJob], error) {
	if err := validateAdminReport(actor, query); err != nil {
		return ports.AdminReportPage[ports.AdminJob]{}, err
	}
	switch query.Status {
	case "", domain.JobStatusPending, domain.JobStatusAccepted, domain.JobStatusInProgress, domain.JobStatusCompleted, domain.JobStatusCancelled:
		return s.repo.ListJobs(ctx, query)
	default:
		return ports.AdminReportPage[ports.AdminJob]{}, ErrInvalidReportQuery
	}
}

func (s *AdminReportService) ListFlaggedMarketers(ctx context.Context, actor Actor, query ports.AdminReportQuery) (ports.AdminReportPage[ports.FlaggedMarketer], error) {
	if err := validateAdminReport(actor, query); err != nil {
		return ports.AdminReportPage[ports.FlaggedMarketer]{}, err
	}
	return s.repo.ListFlaggedMarketers(ctx, query)
}

func validateAdminReport(actor Actor, query ports.AdminReportQuery) error {
	if !actor.IsAdmin {
		return domain.ErrUserForbidden
	}
	if query.Page < 1 || query.Page > 1000000 || query.PageSize < 1 || query.PageSize > 100 {
		return ErrInvalidReportQuery
	}
	return nil
}
