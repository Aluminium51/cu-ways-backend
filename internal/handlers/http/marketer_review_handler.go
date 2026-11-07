package httpapi

import (
	"context"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type marketerReviewService interface {
	List(context.Context, services.Actor, int32, int, int) (ports.MarketerReviewPage, error)
}

type MarketerReviewHandler struct {
	service marketerReviewService
}

func NewMarketerReviewHandler(service marketerReviewService) *MarketerReviewHandler {
	return &MarketerReviewHandler{service: service}
}

type MarketerReviewItem struct {
	ReviewID  int32     `json:"review_id"`
	JobID     int32     `json:"job_id"`
	Rating    int32     `json:"rating"`
	Comment   *string   `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// MarketerReviewsResponse carries one page of verified reviews. AverageRating is
// null and ReviewCount is 0 when the marketer has no qualifying reviews, so the
// profile can show "No ratings yet" instead of a misleading 0.00.
type MarketerReviewsResponse struct {
	Items         []MarketerReviewItem `json:"items"`
	AverageRating *float64             `json:"average_rating"`
	ReviewCount   int64                `json:"review_count"`
	Page          int                  `json:"page"`
	PageSize      int                  `json:"page_size"`
	Total         int64                `json:"total"`
}

// List returns the verified reviews for the marketer named in the path: reviews
// from Completed Jobs where that marketer held the accepted Offer.
func (h *MarketerReviewHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	marketerID, err := parseUserID(c.Params("id"))
	if err != nil {
		return mapMarketerError(domain.ErrMarketerProfileNotFound)
	}
	page, pageSize, err := parsePageValues(c.Query("page"), c.Query("page_size"), services.DefaultMarketerReviewPage, services.DefaultMarketerReviewPageSize, services.MaxMarketerReviewPageSize)
	if err != nil {
		return validationError(err)
	}
	result, err := h.service.List(c.UserContext(), actor, marketerID, page, pageSize)
	if err != nil {
		return mapMarketerError(err)
	}
	items := make([]MarketerReviewItem, 0, len(result.Items))
	for _, review := range result.Items {
		items = append(items, MarketerReviewItem{
			ReviewID:  review.ReviewID,
			JobID:     review.JobID,
			Rating:    review.Rating,
			Comment:   review.Comment,
			CreatedAt: review.CreatedAt,
		})
	}
	return response.Success(c, fiber.StatusOK, MarketerReviewsResponse{
		Items:         items,
		AverageRating: result.AverageRating,
		ReviewCount:   result.Total,
		Page:          result.Page,
		PageSize:      result.PageSize,
		Total:         result.Total,
	})
}
