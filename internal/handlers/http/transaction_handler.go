package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type transactionService interface {
	List(context.Context, services.Actor, ports.TransactionQuery) (ports.TransactionPage, error)
}

type TransactionHandler struct{ service transactionService }

func NewTransactionHandler(service transactionService) *TransactionHandler {
	return &TransactionHandler{service: service}
}

type TransactionParty struct {
	UserID int32  `json:"user_id"`
	Name   string `json:"name"`
}

type TransactionItem struct {
	JobID         int32                `json:"job_id"`
	JobTitle      string               `json:"job_title"`
	Creator       TransactionParty     `json:"creator"`
	Marketer      TransactionParty     `json:"marketer"`
	AcceptedPrice string               `json:"accepted_price"`
	PaymentMethod domain.PaymentMethod `json:"payment_method"`
	PaidAt        *time.Time           `json:"paid_at"`
	PayoutStatus  string               `json:"payout_status"`
}

type TransactionListResponse struct {
	Items    []TransactionItem `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int64             `json:"total"`
}

// List returns the signed-in user's Completed Jobs with their financial
// breakdown. The user comes from the token; the query cannot name another user.
func (h *TransactionHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	page, pageSize, err := parsePageValues(c.Query("page"), c.Query("page_size"), services.DefaultTransactionPage, services.DefaultTransactionPageSize, services.MaxTransactionPageSize)
	if err != nil {
		return validationError(err)
	}
	query := ports.TransactionQuery{
		Role:     domain.TransactionRole(strings.TrimSpace(c.Query("role"))),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	}
	result, err := h.service.List(c.UserContext(), actor, query)
	if err != nil {
		return mapTransactionError(err)
	}
	items := make([]TransactionItem, 0, len(result.Items))
	for _, t := range result.Items {
		items = append(items, TransactionItem{
			JobID:         t.JobID,
			JobTitle:      t.JobTitle,
			Creator:       TransactionParty{UserID: t.CreatorID, Name: t.CreatorName},
			Marketer:      TransactionParty{UserID: t.MarketerID, Name: t.MarketerName},
			AcceptedPrice: t.AcceptedPrice.StringFixed(2),
			PaymentMethod: t.PaymentMethod,
			PaidAt:        t.PaidAt,
			PayoutStatus:  t.PayoutStatus,
		})
	}
	return response.Success(c, fiber.StatusOK, TransactionListResponse{Items: items, Page: result.Page, PageSize: result.PageSize, Total: result.Total})
}

func mapTransactionError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidTransactionQuery):
		return validationError(err)
	case errors.Is(err, domain.ErrUserForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this resource", err)
	default:
		return err
	}
}
