package services

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type StatisticsService struct {
	statistics  ports.MarketerStatisticsRepository
	memberships ports.MembershipRepository
}

func NewStatisticsService(statistics ports.MarketerStatisticsRepository, memberships ports.MembershipRepository) *StatisticsService {
	return &StatisticsService{statistics: statistics, memberships: memberships}
}

func (s *StatisticsService) GetMyStatistics(ctx context.Context, actor Actor) (domain.MarketerStatistics, error) {
	if actor.UserID < 1 {
		return domain.MarketerStatistics{}, domain.ErrServiceForbidden
	}
	isMarketer, err := s.memberships.IsMarketer(ctx, actor.UserID)
	if err != nil {
		return domain.MarketerStatistics{}, err
	}
	if !isMarketer {
		return domain.MarketerStatistics{}, domain.ErrMarketerRequired
	}
	return s.statistics.GetByMarketer(ctx, actor.UserID)
}
