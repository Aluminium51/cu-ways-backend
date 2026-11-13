package ports

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
)

// ProofRepository stores Proof of delivery attachments. Create locks the Job
// so its status and accepted offer cannot change between the checks and the
// insert.
type ProofRepository interface {
	CreateForAcceptedMarketer(ctx context.Context, marketerID, jobID int32, attachment *domain.Attachment) error
	ListForParticipant(ctx context.Context, userID, jobID int32) ([]domain.Attachment, error)
}
