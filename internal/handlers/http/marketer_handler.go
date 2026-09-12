package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/shopspring/decimal"
)

type marketerService interface {
	GetProfile(context.Context, services.Actor) (*domain.Marketer, error)
	SaveProfile(context.Context, services.Actor, services.MarketerProfileInput) (*domain.Marketer, error)
	Search(context.Context, services.Actor, ports.MarketerSearchQuery) (ports.MarketerPage, error)
}

type MarketerHandler struct {
	service marketerService
}

func NewMarketerHandler(service marketerService) *MarketerHandler {
	return &MarketerHandler{service: service}
}

type MarketerProfileDTO struct {
	Bio                string   `json:"bio" validate:"max=5000"`
	ExperienceYears    *int32   `json:"experience_years" validate:"omitempty,gte=0,lte=80"`
	AvailabilityStatus string   `json:"availability_status" validate:"required,oneof=available limited unavailable"`
	AvailabilityText   string   `json:"availability_text" validate:"max=5000"`
	Expertise          []string `json:"expertise" validate:"dive,max=80"`
	Campuses           []string `json:"campuses" validate:"dive,max=80"`
}

type CatalogOptionResponse struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type MarketerProfileResponse struct {
	UserID             int32                     `json:"user_id"`
	Name               string                    `json:"name"`
	Email              string                    `json:"email"`
	Phone              *string                   `json:"phone"`
	LineID             *string                   `json:"line_id"`
	Bio                string                    `json:"bio"`
	ExperienceYears    int32                     `json:"experience_years"`
	AvailabilityStatus domain.AvailabilityStatus `json:"availability_status"`
	AvailabilityText   string                    `json:"availability_text"`
	Expertise          []CatalogOptionResponse   `json:"expertise"`
	Campuses           []CatalogOptionResponse   `json:"campuses"`
	CreatedAt          time.Time                 `json:"created_at"`
}

type MarketerSearchItem struct {
	Profile       MarketerProfileResponse `json:"profile"`
	LowestPrice   *string                 `json:"lowest_matching_service_price"`
	AverageRating *float64                `json:"average_rating"`
	ReviewCount   int64                   `json:"review_count"`
	Services      []ServiceResponse       `json:"services"`
}

type MarketerSearchResponse struct {
	Items    []MarketerSearchItem `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int64                `json:"total"`
}

func (h *MarketerHandler) GetProfile(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	profile, err := h.service.GetProfile(c.UserContext(), actor)
	if err != nil {
		return mapMarketerError(err)
	}
	return response.Success(c, fiber.StatusOK, toMarketerProfileResponse(profile))
}

func (h *MarketerHandler) SaveProfile(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	var dto MarketerProfileDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	var experienceYears int32
	if dto.ExperienceYears != nil {
		experienceYears = *dto.ExperienceYears
	}
	input := services.MarketerProfileInput{
		Bio:                dto.Bio,
		ExperienceYears:    experienceYears,
		AvailabilityStatus: dto.AvailabilityStatus,
		AvailabilityText:   dto.AvailabilityText,
		ExpertiseSlugs:     dto.Expertise,
		CampusSlugs:        dto.Campuses,
	}
	profile, err := h.service.SaveProfile(c.UserContext(), actor, input)
	if err != nil {
		return mapMarketerError(err)
	}
	return response.Success(c, fiber.StatusOK, toMarketerProfileResponse(profile))
}

func (h *MarketerHandler) Search(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	query, err := parseMarketerSearchQuery(c)
	if err != nil {
		return validationError(err)
	}
	page, err := h.service.Search(c.UserContext(), actor, query)
	if err != nil {
		return mapMarketerError(err)
	}
	items := make([]MarketerSearchItem, 0, len(page.Items))
	for _, item := range page.Items {
		servicesResponse := make([]ServiceResponse, 0, len(item.Marketer.Services))
		for index := range item.Marketer.Services {
			servicesResponse = append(servicesResponse, toServiceResponse(&item.Marketer.Services[index]))
		}
		items = append(items, MarketerSearchItem{
			Profile:       toMarketerProfileResponse(&item.Marketer),
			LowestPrice:   item.LowestPrice,
			AverageRating: item.AverageRating,
			ReviewCount:   item.ReviewCount,
			Services:      servicesResponse,
		})
	}
	return response.Success(c, fiber.StatusOK, MarketerSearchResponse{Items: items, Page: page.Page, PageSize: page.PageSize, Total: page.Total})
}

func parseMarketerSearchQuery(c *fiber.Ctx) (ports.MarketerSearchQuery, error) {
	page, pageSize, err := parsePageValues(c.Query("page"), c.Query("page_size"), services.DefaultMarketerPage, services.DefaultMarketerPageSize, services.MaxMarketerPageSize)
	if err != nil {
		return ports.MarketerSearchQuery{}, err
	}
	query := ports.MarketerSearchQuery{Page: page, PageSize: pageSize, Sort: strings.TrimSpace(c.Query("sort"))}
	if raw := strings.TrimSpace(c.Query("min_price")); raw != "" {
		price, err := decimal.NewFromString(raw)
		if err != nil {
			return ports.MarketerSearchQuery{}, errors.New("min_price must be a valid decimal")
		}
		query.MinPrice = &price
	}
	if raw := strings.TrimSpace(c.Query("max_price")); raw != "" {
		price, err := decimal.NewFromString(raw)
		if err != nil {
			return ports.MarketerSearchQuery{}, errors.New("max_price must be a valid decimal")
		}
		query.MaxPrice = &price
	}
	if raw := strings.TrimSpace(c.Query("min_rating")); raw != "" {
		rating, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return ports.MarketerSearchQuery{}, errors.New("min_rating must be a valid number")
		}
		query.MinRating = &rating
	}
	if raw := strings.TrimSpace(c.Query("min_experience_years")); raw != "" {
		years, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return ports.MarketerSearchQuery{}, errors.New("min_experience_years must be an integer")
		}
		value := int32(years)
		query.MinExperienceYears = &value
	}
	if raw := strings.TrimSpace(c.Query("availability_status")); raw != "" {
		status := domain.AvailabilityStatus(strings.ToLower(raw))
		query.AvailabilityStatus = &status
	}
	query.ExpertiseSlugs = queryStrings(c.Context().QueryArgs().PeekMulti("expertise"))
	query.CampusSlugs = queryStrings(c.Context().QueryArgs().PeekMulti("campus"))
	return query, nil
}

func queryStrings(values [][]byte) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func parsePageValues(rawPage, rawPageSize string, defaultPage, defaultPageSize, maxPageSize int) (int, int, error) {
	page, pageSize := defaultPage, defaultPageSize
	if strings.TrimSpace(rawPage) != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New("page must be a positive integer")
		}
		page = parsed
	}
	if strings.TrimSpace(rawPageSize) != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil || parsed < 1 || parsed > maxPageSize {
			return 0, 0, errors.New("page_size is out of range")
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func toMarketerProfileResponse(profile *domain.Marketer) MarketerProfileResponse {
	result := MarketerProfileResponse{
		UserID:             profile.UserID,
		Bio:                profile.Bio,
		ExperienceYears:    profile.ExperienceYears,
		AvailabilityStatus: profile.AvailabilityStatus,
		AvailabilityText:   profile.AvailabilityText,
		Expertise:          make([]CatalogOptionResponse, 0, len(profile.Expertise)),
		Campuses:           make([]CatalogOptionResponse, 0, len(profile.Campuses)),
	}
	if profile.User != nil {
		result.Name = profile.User.Name
		result.Email = profile.User.Email
		result.Phone = profile.User.Phone
		result.LineID = profile.User.LineID
		result.CreatedAt = profile.User.CreatedAt
	}
	for _, option := range profile.Expertise {
		result.Expertise = append(result.Expertise, CatalogOptionResponse{Slug: option.Slug, Name: option.Name})
	}
	for _, option := range profile.Campuses {
		result.Campuses = append(result.Campuses, CatalogOptionResponse{Slug: option.Slug, Name: option.Name})
	}
	return result
}

func mapMarketerError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketerProfileNotFound):
		return response.NewAppError(fiber.StatusNotFound, "marketer_profile_not_found", "marketer profile not found", err)
	case errors.Is(err, domain.ErrMarketerRequired):
		return response.NewAppError(fiber.StatusForbidden, "marketer_profile_required", "a marketer profile is required", err)
	case errors.Is(err, domain.ErrCreatorRequired):
		return response.NewAppError(fiber.StatusForbidden, "creator_required", "a creator profile is required", err)
	case errors.Is(err, domain.ErrCatalogOptionNotFound):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "catalog_option_not_found", "one or more profile options are invalid", err)
	case errors.Is(err, domain.ErrInvalidAvailability):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_availability", "availability status is invalid", err)
	case errors.Is(err, domain.ErrInvalidPrice):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_price", "price filter is invalid", err)
	case errors.Is(err, domain.ErrInvalidMarketerProfile):
		return validationError(err)
	default:
		return err
	}
}
