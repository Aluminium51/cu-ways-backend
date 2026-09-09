package httpapi

import (
	"context"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type statisticsService interface {
	GetMyStatistics(context.Context, services.Actor) (domain.MarketerStatistics, error)
}

type StatisticsHandler struct {
	service statisticsService
}

func NewStatisticsHandler(service statisticsService) *StatisticsHandler {
	return &StatisticsHandler{service: service}
}

type MarketerStatisticsResponse struct {
	TotalCompletedJobs int64    `json:"total_completed_jobs"`
	AverageRating      *float64 `json:"average_rating"`
	TotalEarnings      string   `json:"total_earnings"`
}

func (h *StatisticsHandler) GetMyStatistics(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	statistics, err := h.service.GetMyStatistics(c.UserContext(), actor)
	if err != nil {
		return mapServiceError(err)
	}
	return response.Success(c, fiber.StatusOK, MarketerStatisticsResponse{
		TotalCompletedJobs: statistics.TotalCompletedJobs,
		AverageRating:      statistics.AverageRating,
		TotalEarnings:      statistics.TotalEarnings.StringFixed(2),
	})
}
