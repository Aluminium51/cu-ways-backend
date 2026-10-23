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
	// StartJob runs in one transaction that locks the job. A missing job
	// returns domain.ErrJobNotFound. Unless the job's accepted offer belongs
	// to marketerID it returns domain.ErrJobForbidden, without revealing the
	// job's status. An accepted offer from another job, or one that is not
	// Accepted, returns domain.ErrAcceptedOfferMismatch. A job that is not
	// Accepted returns domain.ErrJobNotAccepted. Otherwise the job becomes
	// In Progress.
	StartJob(ctx context.Context, marketerID, jobID int32) (*StartedJob, error)
}
