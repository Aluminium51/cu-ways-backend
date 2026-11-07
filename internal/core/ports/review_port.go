package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

type ReviewRepository interface {
	CreateForCreatorJob(ctx context.Context, creatorID, jobID int32, review *domain.Review) (*domain.Review, error)
	// FindForCreatorJob checks Job ownership and returns the Job's review, or
	// nil when it has none. reviews.job_id is unique, so there is at most one.
	FindForCreatorJob(ctx context.Context, creatorID, jobID int32) (*domain.Review, error)
}
