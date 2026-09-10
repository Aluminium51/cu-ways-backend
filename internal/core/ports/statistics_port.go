package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type MarketerStatisticsRepository interface {
	GetByMarketer(ctx context.Context, userID int32) (domain.MarketerStatistics, error)
}
