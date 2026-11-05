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

type jobSettlementService interface {
	GetPayment(context.Context, services.Actor, int32) (*domain.JobSettlement, error)
	Complete(context.Context, services.Actor, int32) (*domain.JobSettlement, error)
	GetReview(context.Context, services.Actor, int32) (*domain.Review, error)
}

type JobSettlementHandler struct{ service jobSettlementService }

func NewJobSettlementHandler(service jobSettlementService) *JobSettlementHandler {
	return &JobSettlementHandler{service: service}
}

type JobSettlementResponse struct {
	JobID         int32            `json:"job_id"`
	JobTitle      string           `json:"job_title"`
	JobStatus     domain.JobStatus `json:"job_status"`
	Creator       TransactionParty `json:"creator"`
	Marketer      TransactionParty `json:"marketer"`
	AcceptedPrice string           `json:"accepted_price"`
	Payment       *PaymentResponse `json:"payment"`
	PayoutStatus  *string          `json:"payout_status"`
	CompletedAt   *time.Time       `json:"completed_at"`
}

func (h *JobSettlementHandler) GetPayment(c *fiber.Ctx) error {
	return respondWithSettlement(c, h.service.GetPayment)
}

func (h *JobSettlementHandler) Complete(c *fiber.Ctx) error {
	return respondWithSettlement(c, h.service.Complete)
}

func respondWithSettlement(c *fiber.Ctx, load func(context.Context, services.Actor, int32) (*domain.JobSettlement, error)) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	record, err := load(c.UserContext(), actor, jobID)
	if err != nil {
		return mapSettlementError(err)
	}
	result := JobSettlementResponse{
		JobID: record.JobID, JobTitle: record.JobTitle, JobStatus: record.JobStatus,
		Creator:       TransactionParty{UserID: record.CreatorID, Name: record.CreatorName},
		Marketer:      TransactionParty{UserID: record.MarketerID, Name: record.MarketerName},
		AcceptedPrice: record.AcceptedPrice.StringFixed(2), CompletedAt: record.CompletedAt,
	}
	if record.Payment != nil {
		payment, payout := paymentResponse(record.Payment), record.PayoutStatus()
		result.Payment, result.PayoutStatus = &payment, &payout
	}
	return response.Success(c, fiber.StatusOK, result)
}

func (h *JobSettlementHandler) GetReview(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	review, err := h.service.GetReview(c.UserContext(), actor, jobID)
	if err != nil {
		return mapReviewError(err)
	}
	var result *ReviewResponse
	if review != nil {
		saved := reviewResponse(review)
		result = &saved
	}
	return response.Success(c, fiber.StatusOK, struct {
		Review *ReviewResponse `json:"review"`
	}{Review: result})
}

func mapSettlementError(err error) error {
	switch {
	case errors.Is(err, domain.ErrJobNotInProgress):
		return response.NewAppError(fiber.StatusConflict, "job_not_in_progress", "only in-progress jobs can be completed", err)
	case errors.Is(err, domain.ErrCompletionPaymentRequired):
		return response.NewAppError(fiber.StatusConflict, "completion_payment_required", err.Error(), err)
	case errors.Is(err, domain.ErrCompletionProofRequired):
		return response.NewAppError(fiber.StatusConflict, "completion_proof_required", err.Error(), err)
	default:
		return mapPaymentError(err)
	}
}
