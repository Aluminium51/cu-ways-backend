package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// StartedJob is a job the hired Marketer has begun, with the accepted offer
// that hired them.
type StartedJob struct {
	Job   domain.Job
	Offer domain.Offer
}

// MarketerJobRepository exposes the persistence operations the hired
// Marketer needs to run a job.
type MarketerJobRepository interface {
	// StartJob runs in one transaction that locks the job, applies
	// domain.Job.Start for marketerID, and persists the new status. A missing
	// job returns domain.ErrJobNotFound; every other refusal is the error
	// domain.Job.Start returns.
	StartJob(ctx context.Context, marketerID, jobID int32) (*StartedJob, error)
}
