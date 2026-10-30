package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type reviewService interface {
	Submit(context.Context, services.Actor, int32, services.SubmitReviewInput) (*domain.Review, error)
}

type ReviewHandler struct {
	service reviewService
}

func NewReviewHandler(service reviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

type SubmitReviewDTO struct {
	Rating  int32   `json:"rating" validate:"required,gte=1,lte=5"`
	Comment *string `json:"comment"`
}

type ReviewResponse struct {
	ReviewID  int32     `json:"review_id"`
	JobID     int32     `json:"job_id"`
	Rating    int32     `json:"rating"`
	Comment   *string   `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *ReviewHandler) Submit(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto SubmitReviewDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	review, err := h.service.Submit(c.UserContext(), actor, jobID, services.SubmitReviewInput{Rating: dto.Rating, Comment: dto.Comment})
	if err != nil {
		return mapReviewError(err)
	}
	return response.Success(c, fiber.StatusCreated, ReviewResponse{
		ReviewID: review.ReviewID, JobID: review.JobID, Rating: review.Rating,
		Comment: review.Comment, CreatedAt: review.CreatedAt,
	})
}

func mapReviewError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCreatorRequired):
		return response.NewAppError(fiber.StatusForbidden, "creator_required", "a creator profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this job", err)
	case errors.Is(err, domain.ErrJobNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_not_found", "job was not found", err)
	case errors.Is(err, domain.ErrJobNotCompleted):
		return response.NewAppError(fiber.StatusConflict, "job_not_completed", "only completed jobs can be reviewed", err)
	case errors.Is(err, domain.ErrReviewAlreadyExists):
		return response.NewAppError(fiber.StatusConflict, "review_already_exists", "this job already has a review", err)
	case errors.Is(err, domain.ErrInvalidJob), errors.Is(err, domain.ErrInvalidReview):
		return validationError(err)
	default:
		return err
	}
}
