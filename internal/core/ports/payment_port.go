package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// PaymentRepository validates the locked Job/offer snapshot and inserts the
// payment atomically, serializing submissions for the same Job.
type PaymentRepository interface {
	CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, payment *domain.Payment) (*domain.Payment, error)
}
