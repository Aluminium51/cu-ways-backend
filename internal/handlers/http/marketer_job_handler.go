package httpapi

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type marketerJobService interface {
	StartJob(context.Context, services.Actor, int32) (*ports.StartedJob, error)
}

type MarketerJobHandler struct {
	service marketerJobService
}

func NewMarketerJobHandler(service marketerJobService) *MarketerJobHandler {
	return &MarketerJobHandler{service: service}
}

// StartJobResponse reports the started job and the accepted offer that hired
// the Marketer.
type StartJobResponse struct {
	JobID           int32            `json:"job_id"`
	JobStatus       domain.JobStatus `json:"job_status"`
	AcceptedOfferID int32            `json:"accepted_offer_id"`
	Offer           OfferResponse    `json:"offer"`
}

func (h *MarketerJobHandler) Start(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parsePathID(c.Params("id"), "job")
	if err != nil {
		return validationError(err)
	}
	result, err := h.service.StartJob(c.UserContext(), actor, jobID)
	if err != nil {
		return mapMarketerJobError(err)
	}
	return response.Success(c, fiber.StatusOK, StartJobResponse{
		JobID:           result.Job.JobID,
		JobStatus:       result.Job.JobStatus,
		AcceptedOfferID: result.Offer.OfferID,
		Offer:           toOfferResponse(&result.Offer),
	})
}

func mapMarketerJobError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketerRequired):
		return response.NewAppError(fiber.StatusForbidden, "marketer_profile_required", "a marketer profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden):
		return response.NewAppError(fiber.StatusForbidden, "job_forbidden", "only the marketer hired through the accepted offer can start this job", err)
	case errors.Is(err, domain.ErrJobNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_not_found", "job was not found", err)
	case errors.Is(err, domain.ErrJobNotAccepted):
		return response.NewAppError(fiber.StatusConflict, "job_not_accepted", "only an accepted job can be started", err)
	case errors.Is(err, domain.ErrAcceptedOfferMismatch):
		return response.NewAppError(fiber.StatusConflict, "accepted_offer_mismatch", "the job's accepted offer is no longer accepted", err)
	case errors.Is(err, domain.ErrInvalidJob):
		return validationError(err)
	default:
		return err
	}
}
