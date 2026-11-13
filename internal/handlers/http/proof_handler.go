package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type proofService interface {
	Add(context.Context, services.Actor, int32, services.AddProofInput) (*domain.Attachment, error)
	List(context.Context, services.Actor, int32) ([]domain.Attachment, error)
}

type ProofHandler struct{ service proofService }

func NewProofHandler(service proofService) *ProofHandler {
	return &ProofHandler{service: service}
}

type CreateProofDTO struct {
	URL            string                `json:"url"`
	AttachmentType domain.AttachmentType `json:"attachment_type"`
}

type ProofResponse struct {
	AttachmentID   int32                 `json:"attachment_id"`
	JobID          int32                 `json:"job_id"`
	AttachmentType domain.AttachmentType `json:"attachment_type"`
	URL            string                `json:"url"`
	UploadAt       time.Time             `json:"upload_at"`
}

func (h *ProofHandler) Create(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto CreateProofDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	attachment, err := h.service.Add(c.UserContext(), actor, jobID, services.AddProofInput{URL: dto.URL, AttachmentType: dto.AttachmentType})
	if err != nil {
		return mapProofError(err)
	}
	return response.Success(c, fiber.StatusCreated, toProofResponse(*attachment))
}

func (h *ProofHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	attachments, err := h.service.List(c.UserContext(), actor, jobID)
	if err != nil {
		return mapProofError(err)
	}
	items := make([]ProofResponse, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, toProofResponse(attachment))
	}
	return response.Success(c, fiber.StatusOK, items)
}

func toProofResponse(a domain.Attachment) ProofResponse {
	return ProofResponse{AttachmentID: a.AttachmentID, JobID: a.JobID, AttachmentType: a.AttachmentType, URL: a.URL, UploadAt: a.UploadAt}
}

func mapProofError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketerRequired):
		return response.NewAppError(fiber.StatusForbidden, "marketer_required", "a marketer profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "only the job's creator or hired marketer can access its proof", err)
	case errors.Is(err, domain.ErrJobNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_not_found", "job was not found", err)
	case errors.Is(err, domain.ErrJobNotInProgress):
		return response.NewAppError(fiber.StatusConflict, "job_not_in_progress", err.Error(), err)
	case errors.Is(err, domain.ErrAcceptedOfferMismatch):
		return response.NewAppError(fiber.StatusConflict, "accepted_offer_mismatch", err.Error(), err)
	case errors.Is(err, domain.ErrInvalidAttachmentType):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_attachment_type", err.Error(), err)
	case errors.Is(err, domain.ErrInvalidProofURL):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_proof_url", err.Error(), err)
	case errors.Is(err, domain.ErrInvalidJob):
		return validationError(err)
	default:
		return err
	}
}
