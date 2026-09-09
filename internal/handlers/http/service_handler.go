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

type serviceService interface {
	List(context.Context, services.Actor) ([]domain.Service, error)
	Create(context.Context, services.Actor, services.CreateServiceInput) (*domain.Service, error)
	Update(context.Context, services.Actor, int32, services.UpdateServiceInput) (*domain.Service, error)
	Delete(context.Context, services.Actor, int32) error
}

type ServiceHandler struct {
	service serviceService
}

func NewServiceHandler(service serviceService) *ServiceHandler {
	return &ServiceHandler{service: service}
}

type CreateServiceDTO struct {
	ServiceType string  `json:"service_type" validate:"required,max=100"`
	ScopeText   *string `json:"scope_text" validate:"omitempty,max=5000"`
	Price       string  `json:"price" validate:"required"`
}

type UpdateServiceDTO struct {
	ServiceType OptionalString `json:"service_type"`
	ScopeText   OptionalString `json:"scope_text"`
	Price       OptionalString `json:"price"`
}

type ServiceResponse struct {
	ServiceID   int32     `json:"service_id"`
	ServiceType string    `json:"service_type"`
	ScopeText   *string   `json:"scope_text"`
	Price       string    `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DeleteServiceResponse struct {
	ServiceID int32 `json:"service_id"`
	Deleted   bool  `json:"deleted"`
}

func (h *ServiceHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	items, err := h.service.List(c.UserContext(), actor)
	if err != nil {
		return mapServiceError(err)
	}
	result := make([]ServiceResponse, 0, len(items))
	for index := range items {
		result = append(result, toServiceResponse(&items[index]))
	}
	return response.Success(c, fiber.StatusOK, result)
}

func (h *ServiceHandler) Create(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	var dto CreateServiceDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	item, err := h.service.Create(c.UserContext(), actor, services.CreateServiceInput{
		ServiceType: dto.ServiceType,
		ScopeText:   dto.ScopeText,
		Price:       dto.Price,
	})
	if err != nil {
		return mapServiceError(err)
	}
	return response.Success(c, fiber.StatusCreated, toServiceResponse(item))
}

func (h *ServiceHandler) Update(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	serviceID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	var dto UpdateServiceDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if !dto.ServiceType.Set && !dto.ScopeText.Set && !dto.Price.Set {
		return validationError(domain.ErrNoServiceChanges)
	}
	if dto.ServiceType.Set && dto.ServiceType.Value == nil {
		return validationError(domain.ErrInvalidService)
	}
	if dto.Price.Set && dto.Price.Value == nil {
		return validationError(domain.ErrInvalidPrice)
	}
	item, err := h.service.Update(c.UserContext(), actor, serviceID, services.UpdateServiceInput{
		ServiceTypeSet: dto.ServiceType.Set,
		ServiceType:    optionalStringValue(dto.ServiceType),
		ScopeTextSet:   dto.ScopeText.Set,
		ScopeText:      dto.ScopeText.Value,
		PriceSet:       dto.Price.Set,
		Price:          optionalStringValue(dto.Price),
	})
	if err != nil {
		return mapServiceError(err)
	}
	return response.Success(c, fiber.StatusOK, toServiceResponse(item))
}

func (h *ServiceHandler) Delete(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	serviceID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	if err := h.service.Delete(c.UserContext(), actor, serviceID); err != nil {
		return mapServiceError(err)
	}
	return response.Success(c, fiber.StatusOK, DeleteServiceResponse{ServiceID: serviceID, Deleted: true})
}

func optionalStringValue(value OptionalString) string {
	if value.Value == nil {
		return ""
	}
	return *value.Value
}

func toServiceResponse(service *domain.Service) ServiceResponse {
	return ServiceResponse{
		ServiceID:   service.ServiceID,
		ServiceType: service.ServiceType,
		ScopeText:   service.ScopeText,
		Price:       service.Price.StringFixed(2),
		CreatedAt:   service.CreatedAt,
		UpdatedAt:   service.UpdatedAt,
	}
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, domain.ErrMarketerRequired):
		return response.NewAppError(fiber.StatusForbidden, "marketer_profile_required", "a marketer profile is required", err)
	case errors.Is(err, domain.ErrServiceForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this service", err)
	case errors.Is(err, domain.ErrServiceNotFound):
		return response.NewAppError(fiber.StatusNotFound, "service_not_found", "service not found", err)
	case errors.Is(err, domain.ErrInvalidPrice):
		return response.NewAppError(fiber.StatusUnprocessableEntity, "invalid_price", "price must be a non-negative decimal with at most two decimal places", err)
	case errors.Is(err, domain.ErrInvalidService), errors.Is(err, domain.ErrNoServiceChanges):
		return validationError(err)
	default:
		return err
	}
}
