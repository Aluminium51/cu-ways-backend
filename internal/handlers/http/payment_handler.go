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

type paymentService interface {
	Create(context.Context, services.Actor, int32, services.CreatePaymentInput) (*domain.Payment, error)
}

type PaymentHandler struct{ service paymentService }

func NewPaymentHandler(service paymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

type CreatePaymentDTO struct {
	Method domain.PaymentMethod `json:"method"`
	Amount string               `json:"amount"`
}

type PaymentResponse struct {
	PaymentID     int32                `json:"payment_id"`
	JobID         int32                `json:"job_id"`
	Method        domain.PaymentMethod `json:"method"`
	Amount        string               `json:"amount"`
	PaymentStatus domain.PaymentStatus `json:"payment_status"`
	CreatedAt     *time.Time           `json:"created_at"`
	PaidAt        *time.Time           `json:"paid_at"`
}

func (h *PaymentHandler) Create(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	jobID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto CreatePaymentDTO
	if err := decodeStrictJSON(c, &dto); err != nil {
		return validationError(err)
	}
	p, err := h.service.Create(c.UserContext(), actor, jobID, services.CreatePaymentInput{Method: dto.Method, Amount: dto.Amount})
	if err != nil {
		return mapPaymentError(err)
	}
	return response.Success(c, fiber.StatusCreated, PaymentResponse{PaymentID: p.PaymentID, JobID: p.JobID, Method: p.Method, Amount: p.Amount.StringFixed(2), PaymentStatus: p.PaymentStatus, CreatedAt: p.CreatedAt, PaidAt: p.PaidAt})
}

func mapPaymentError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCreatorRequired):
		return response.NewAppError(fiber.StatusForbidden, "creator_required", "a creator profile is required", err)
	case errors.Is(err, domain.ErrJobForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this job", err)
	case errors.Is(err, domain.ErrJobNotFound):
		return response.NewAppError(fiber.StatusNotFound, "job_not_found", "job was not found", err)
	case errors.Is(err, domain.ErrJobNotPayable):
		return response.NewAppError(fiber.StatusConflict, "job_not_payable", err.Error(), err)
	case errors.Is(err, domain.ErrAcceptedOfferRequired):
		return response.NewAppError(fiber.StatusConflict, "accepted_offer_required", err.Error(), err)
	case errors.Is(err, domain.ErrPaymentAlreadyExists):
		return response.NewAppError(fiber.StatusConflict, "payment_already_exists", err.Error(), err)
	case errors.Is(err, domain.ErrPaymentAmountMismatch):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "payment_amount_mismatch", err.Error(), err)
	case errors.Is(err, domain.ErrInvalidJob), errors.Is(err, domain.ErrInvalidPayment):
		return validationError(err)
	default:
		return err
	}
}
