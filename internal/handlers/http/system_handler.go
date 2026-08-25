package httpapi

import (
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type HealthHandler struct {
	service *services.HealthService
}

func NewHealthHandler(service *services.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Health(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, fiber.Map{"status": "ok"})
}

func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	if err := h.service.Check(c.UserContext()); err != nil {
		return response.NewAppError(fiber.StatusServiceUnavailable, "database_unavailable", "service is not ready", err)
	}
	return response.Success(c, fiber.StatusOK, fiber.Map{"status": "ready"})
}
