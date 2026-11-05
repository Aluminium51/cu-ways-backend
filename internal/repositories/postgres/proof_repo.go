package postgres

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProofRepository struct{ db *gorm.DB }

var _ ports.ProofRepository = (*ProofRepository)(nil)

func NewProofRepository(db *gorm.DB) *ProofRepository { return &ProofRepository{db: db} }

func (r *ProofRepository) CreateForAcceptedMarketer(ctx context.Context, marketerID, jobID int32, attachment *domain.Attachment) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("job_id = ?", jobID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrJobNotFound
			}
			return err
		}
		accepted, err := isAcceptedMarketer(tx, job, marketerID)
		if err != nil {
			return err
		}
		if !accepted {
			return domain.ErrJobForbidden
		}
		if job.JobStatus != domain.JobStatusInProgress {
			return domain.ErrJobNotInProgress
		}
		attachment.JobID = jobID
		return tx.Create(attachment).Error
	})
	return mapProofRepositoryError(err)
}

func (r *ProofRepository) ListForParticipant(ctx context.Context, userID, jobID int32) ([]domain.Attachment, error) {
	db := r.db.WithContext(ctx)
	var job domain.Job
	if err := db.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrJobNotFound
		}
		return nil, err
	}
	if job.UserID != userID {
		accepted, err := isAcceptedMarketer(db, job, userID)
		if err != nil {
			return nil, err
		}
		if !accepted {
			return nil, domain.ErrJobForbidden
		}
	}
	attachments := []domain.Attachment{}
	err := db.Where("job_id = ? AND attachment_type = ?", jobID, domain.AttachmentTypeProof).
		Order("upload_at DESC, attachment_id DESC").
		Find(&attachments).Error
	return attachments, err
}

// isAcceptedMarketer reports whether userID made the job's accepted offer.
func isAcceptedMarketer(tx *gorm.DB, job domain.Job, userID int32) (bool, error) {
	if job.AcceptedOfferID == nil {
		return false, nil
	}
	var count int64
	err := tx.Model(&domain.Offer{}).
		Where("job_id = ? AND offer_id = ? AND user_id = ?", job.JobID, *job.AcceptedOfferID, userID).
		Count(&count).Error
	return count > 0, err
}

func mapProofRepositoryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "chk_attachment_type" {
		return domain.ErrInvalidAttachmentType
	}
	return err
}
