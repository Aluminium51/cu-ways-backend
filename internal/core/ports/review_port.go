package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type ReviewRepository interface {
	CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, review *domain.Review) (*domain.Review, error)
}
