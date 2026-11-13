package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type JobSettlementRepository interface {
	GetPayment(ctx context.Context, userID, jobID int32) (*domain.JobSettlement, error)
	Complete(ctx context.Context, creatorID, jobID int32) (*domain.JobSettlement, error)
}
