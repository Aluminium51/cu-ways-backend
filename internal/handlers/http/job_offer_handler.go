package httpapi

import (
	"context"
	"errors"
	"strings"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type jobOfferService interface {
	ListOffers(context.Context, services.Actor, int32, *domain.OfferStatus) ([]ports.JobOfferComparison, error)
	AcceptOffer(context.Context, services.Actor, int32, int32) (*ports.OfferAcceptance, error)
}

type JobOfferHandler struct {
	service jobOfferService
}

func NewJobOfferHandler(service jobOfferService) *JobOfferHandler {
	return &JobOfferHandler{service: service}
}

type JobOfferComparisonResponse struct {
	OfferResponse
	Marketer MarketerProfileResponse `json:"marketer"`
	Rating   MarketerRatingResponse  `json:"rating"`
}

// MarketerRatingResponse reports a Marketer's history. IsRated is false and
// AverageRating null for Marketers with no rated jobs.
type MarketerRatingResponse struct {
	AverageRating *float64 `json:"average_rating"`
	ReviewCount   int64    `json:"review_count"`
	CompletedJobs int64    `json:"completed_jobs"`
	IsRated       bool     `json:"is_rated"`
}

func (h *JobOfferHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parsePathID(c.Params("id"), "job")
	if err != nil {
		return validationError(err)
	}
	var status *domain.OfferStatus
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		value := domain.OfferStatus(raw)
		if !value.IsValid() {
			return validationError(errors.New("status must be Pending, Accepted, Rejected, or Withdrawn"))
		}
		status = &value
	}
	items, err := h.service.ListOffers(c.UserContext(), actor, jobID, status)
	if err != nil {
		return mapJobOfferError(err)
	}
	result := make([]JobOfferComparisonResponse, 0, len(items))
	for index := range items {
		result = append(result, toJobOfferComparisonResponse(&items[index]))
	}
	return response.Success(c, fiber.StatusOK, result)
}

// AcceptOfferResponse reports the hired offer and every row the acceptance
// changed, so the frontend can refresh job, offer, and request badges.
type AcceptOfferResponse struct {
	JobID               int32            `json:"job_id"`
	JobStatus           domain.JobStatus `json:"job_status"`
	AcceptedOfferID     int32            `json:"accepted_offer_id"`
	Offer               OfferResponse    `json:"offer"`
	RejectedOfferIDs    []int32          `json:"rejected_offer_ids"`
	ClosedJobRequestIDs []int32          `json:"closed_job_request_ids"`
}

func (h *JobOfferHandler) Accept(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parsePathID(c.Params("id"), "job")
	if err != nil {
		return validationError(err)
	}
	offerID, err := parsePathID(c.Params("offerId"), "offer")
	if err != nil {
		return validationError(err)
	}
	result, err := h.service.AcceptOffer(c.UserContext(), actor, jobID, offerID)
	if err != nil {
		return mapJobOfferError(err)
	}
	return response.Success(c, fiber.StatusOK, AcceptOfferResponse{
		JobID:               result.Job.JobID,
		JobStatus:           result.Job.JobStatus,
		AcceptedOfferID:     result.Offer.OfferID,
		Offer:               toOfferResponse(&result.Offer),
		RejectedOfferIDs:    result.RejectedOfferIDs,
		ClosedJobRequestIDs: result.ClosedJobRequestIDs,
	})
}

func toJobOfferComparisonResponse(item *ports.JobOfferComparison) JobOfferComparisonResponse {
	return JobOfferComparisonResponse{
		OfferResponse: toOfferResponse(&item.Offer),
		Marketer:      toMarketerProfileResponse(&item.Marketer),
		Rating: MarketerRatingResponse{
			AverageRating: item.Rating.AverageRating,
			ReviewCount:   item.Rating.ReviewCount,
			CompletedJobs: item.Rating.CompletedJobs,
			IsRated:       item.Rating.AverageRating != nil,
		},
	}
}

func mapJobOfferError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCreatorRequired):
		return response.NewAppError(fiber.StatusForbidden, "creator_required", "a creator profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this job", err)
	case errors.Is(err, domain.ErrJobNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_not_found", "job was not found", err)
	case errors.Is(err, domain.ErrOfferNotFound):
		return response.NewAppError(fiber.StatusNotFound, "offer_not_found", "offer was not found for this job", err)
	case errors.Is(err, domain.ErrJobNotPending):
		return response.NewAppError(fiber.StatusConflict, "job_not_pending", "this job already has an accepted offer or is closed", err)
	case errors.Is(err, domain.ErrOfferNotPending):
		return response.NewAppError(fiber.StatusConflict, "offer_not_pending", "only a pending offer can be accepted", err)
	case errors.Is(err, domain.ErrInvalidJob), errors.Is(err, domain.ErrInvalidOffer):
		return validationError(err)
	default:
		return err
	}
}
