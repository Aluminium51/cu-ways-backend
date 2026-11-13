package services

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
)

type AddProofInput struct {
	URL string
	// AttachmentType is optional; an empty value means Proof.
	AttachmentType domain.AttachmentType
}

type ProofService struct {
	repo        ports.ProofRepository
	memberships ports.MembershipRepository
}

func NewProofService(repo ports.ProofRepository, memberships ports.MembershipRepository) *ProofService {
	return &ProofService{repo: repo, memberships: memberships}
}

// Add saves a delivery link as a Proof attachment for the accepted marketer
// of an In Progress job. The upload time is always set by the server.
func (s *ProofService) Add(ctx context.Context, actor Actor, jobID int32, input AddProofInput) (*domain.Attachment, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrMarketerRequired
	}
	isMarketer, err := s.memberships.IsMarketer(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !isMarketer {
		return nil, domain.ErrMarketerRequired
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	attachmentType := input.AttachmentType
	if attachmentType == "" {
		attachmentType = domain.AttachmentTypeProof
	}
	if attachmentType != domain.AttachmentTypeProof {
		return nil, domain.ErrInvalidAttachmentType
	}
	proofURL, ok := validAttachmentURL(input.URL)
	if !ok {
		return nil, domain.ErrInvalidProofURL
	}
	attachment := &domain.Attachment{
		AttachmentType: attachmentType,
		URL:            proofURL,
		UploadAt:       time.Now().UTC(),
	}
	if err := s.repo.CreateForAcceptedMarketer(ctx, actor.UserID, jobID, attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

// List returns a job's Proof attachments, newest first, to its creator or its
// accepted marketer.
func (s *ProofService) List(ctx context.Context, actor Actor, jobID int32) ([]domain.Attachment, error) {
	if actor.UserID < 1 {
		return nil, domain.ErrJobForbidden
	}
	if jobID < 1 {
		return nil, domain.ErrInvalidJob
	}
	return s.repo.ListForParticipant(ctx, actor.UserID, jobID)
}
