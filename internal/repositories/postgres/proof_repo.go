package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type ProofRepository struct{ db *gorm.DB }

var _ ports.ProofRepository = (*ProofRepository)(nil)

func NewProofRepository(db *gorm.DB) *ProofRepository { return &ProofRepository{db: db} }

func (r *ProofRepository) CreateForAcceptedMarketer(ctx context.Context, marketerID, jobID int32, attachment *domain.Attachment) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := lockJob(tx, jobID)
		if err != nil {
			return err
		}
		accepted, err := findAcceptedOffer(tx, job)
		if err != nil {
			return err
		}
		if err := job.CanAddProof(marketerID, accepted); err != nil {
			return err
		}
		attachment.JobID = jobID
		return tx.Create(attachment).Error
	})
	return mapProofRepositoryError(err)
}

// ListForParticipant reads the access check and the rows from one snapshot.
func (r *ProofRepository) ListForParticipant(ctx context.Context, userID, jobID int32) ([]domain.Attachment, error) {
	attachments := []domain.Attachment{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job domain.Job
		if err := tx.Where("job_id = ?", jobID).First(&job).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrJobNotFound
			}
			return err
		}
		accepted, err := findAcceptedOffer(tx, &job)
		if err != nil {
			return err
		}
		if err := job.CanViewProof(userID, accepted); err != nil {
			return err
		}
		return tx.Where("job_id = ? AND attachment_type = ?", jobID, domain.AttachmentTypeProof).
			Order("upload_at DESC, attachment_id DESC").
			Find(&attachments).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return attachments, nil
}

func mapProofRepositoryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "chk_attachment_type" {
		return domain.ErrInvalidAttachmentType
	}
	return err
}
