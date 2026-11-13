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

type AdminReportHandler struct {
	service *services.AdminReportService
}

func NewAdminReportHandler(service *services.AdminReportService) *AdminReportHandler {
	return &AdminReportHandler{service: service}
}

func (h *AdminReportHandler) ListJobs(c *fiber.Ctx) error {
	return respondWithReport(c, h.service.ListJobs)
}

func (h *AdminReportHandler) ListFlaggedMarketers(c *fiber.Ctx) error {
	return respondWithReport(c, h.service.ListFlaggedMarketers)
}

func respondWithReport[T any](c *fiber.Ctx, load func(context.Context, services.Actor, ports.AdminReportQuery) (T, error)) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	if !actor.IsAdmin {
		return mapUserError(domain.ErrUserForbidden)
	}
	page, err := parsePathID(c.Query("page", "1"), "page")
	if err != nil {
		return validationError(err)
	}
	pageSize, err := parsePathID(c.Query("page_size", "20"), "page size")
	if err != nil {
		return validationError(err)
	}
	data, err := load(c.UserContext(), actor, ports.AdminReportQuery{
		Page: int(page), PageSize: int(pageSize), Status: domain.JobStatus(c.Query("status")),
	})
	if errors.Is(err, services.ErrInvalidReportQuery) {
		return validationError(err)
	}
	if err != nil {
		return mapUserError(err)
	}
	return response.Success(c, fiber.StatusOK, data)
}
