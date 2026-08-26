package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type userService interface {
	Create(context.Context, services.CreateUserInput) (*domain.User, error)
	Get(context.Context, services.Actor, int32) (*domain.User, error)
	List(context.Context, services.Actor, ports.UserListQuery) (ports.UserPage, error)
	Update(context.Context, services.Actor, int32, services.UpdateUserInput) (*domain.User, error)
	Delete(context.Context, services.Actor, int32) (time.Time, error)
}

type UserHandler struct {
	service userService
}

func NewUserHandler(service userService) *UserHandler {
	return &UserHandler{service: service}
}

type CreateUserDTO struct {
	Name   string  `json:"name" validate:"required,max=100"`
	Email  string  `json:"email" validate:"required,email,max=255"`
	Phone  *string `json:"phone" validate:"omitempty,max=20"`
	LineID *string `json:"line_id" validate:"omitempty,max=50"`
}

// OptionalString preserves whether a JSON field was omitted or explicitly
// supplied, including an explicit null value.
type OptionalString struct {
	Set   bool
	Value *string
}

func (o *OptionalString) UnmarshalJSON(data []byte) error {
	o.Set = true
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type UpdateUserDTO struct {
	Name   OptionalString `json:"name"`
	Email  OptionalString `json:"email"`
	Phone  OptionalString `json:"phone"`
	LineID OptionalString `json:"line_id"`
}

type UserResponse struct {
	UserID    int32     `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	LineID    *string   `json:"line_id"`
	CreatedAt time.Time `json:"created_at"`
}

type UserListResponse struct {
	Items    []UserResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}

type DeleteUserResponse struct {
	UserID  int32 `json:"user_id"`
	Deleted bool  `json:"deleted"`
}

func (h *UserHandler) Create(c *fiber.Ctx) error {
	var dto CreateUserDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}
	if err := validateContactDTO(dto.Phone, 20); err != nil {
		return validationError(err)
	}
	if err := validateContactDTO(dto.LineID, 50); err != nil {
		return validationError(err)
	}

	user, err := h.service.Create(c.UserContext(), services.CreateUserInput{
		Name:   dto.Name,
		Email:  dto.Email,
		Phone:  dto.Phone,
		LineID: dto.LineID,
	})
	if err != nil {
		return mapUserError(err)
	}
	return response.Success(c, fiber.StatusCreated, toUserResponse(user))
}

func (h *UserHandler) Get(c *fiber.Ctx) error {
	userID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}

	user, err := h.service.Get(c.UserContext(), actor, userID)
	if err != nil {
		return mapUserError(err)
	}
	return response.Success(c, fiber.StatusOK, toUserResponse(user))
}

func (h *UserHandler) List(c *fiber.Ctx) error {
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}
	page, pageSize, err := parsePagination(c)
	if err != nil {
		return validationError(err)
	}

	result, err := h.service.List(c.UserContext(), actor, ports.UserListQuery{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return mapUserError(err)
	}

	items := make([]UserResponse, 0, len(result.Items))
	for index := range result.Items {
		items = append(items, toUserResponse(&result.Items[index]))
	}
	return response.Success(c, fiber.StatusOK, UserListResponse{
		Items:    items,
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	})
}

func (h *UserHandler) Update(c *fiber.Ctx) error {
	userID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}

	var dto UpdateUserDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := validateUpdateDTO(dto); err != nil {
		return validationError(err)
	}

	input := services.UpdateUserInput{
		PhoneSet:  dto.Phone.Set,
		Phone:     dto.Phone.Value,
		LineIDSet: dto.LineID.Set,
		LineID:    dto.LineID.Value,
	}
	if dto.Name.Set {
		input.Name = dto.Name.Value
	}
	if dto.Email.Set {
		input.Email = dto.Email.Value
	}

	user, err := h.service.Update(c.UserContext(), actor, userID, input)
	if err != nil {
		return mapUserError(err)
	}
	return response.Success(c, fiber.StatusOK, toUserResponse(user))
}

func (h *UserHandler) Delete(c *fiber.Ctx) error {
	userID, err := parseUserID(c.Params("id"))
	if err != nil {
		return validationError(err)
	}
	actor, err := actorFromRequest(c)
	if err != nil {
		return err
	}

	if _, err := h.service.Delete(c.UserContext(), actor, userID); err != nil {
		return mapUserError(err)
	}
	return response.Success(c, fiber.StatusOK, DeleteUserResponse{UserID: userID, Deleted: true})
}

func actorFromRequest(c *fiber.Ctx) (services.Actor, error) {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		return services.Actor{}, response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "authentication required", nil)
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 32)
	if err != nil || userID < 1 {
		return services.Actor{}, response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "invalid authentication token", err)
	}
	role, _ := claims.Values["role"].(string)
	return services.Actor{UserID: int32(userID), IsAdmin: role == "admin"}, nil
}

func parseUserID(value string) (int32, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil || parsed < 1 {
		return 0, errors.New("invalid user id")
	}
	return int32(parsed), nil
}

func parsePagination(c *fiber.Ctx) (int, int, error) {
	page := services.DefaultUserPage
	pageSize := services.DefaultUserPageSize

	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New("page must be a positive integer")
		}
		page = parsed
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > services.MaxUserPageSize {
			return 0, 0, errors.New("page_size must be between 1 and 100")
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func validateUpdateDTO(dto UpdateUserDTO) error {
	if !dto.Name.Set && !dto.Email.Set && !dto.Phone.Set && !dto.LineID.Set {
		return domain.ErrNoUserChanges
	}
	if dto.Name.Set {
		if dto.Name.Value == nil || strings.TrimSpace(*dto.Name.Value) == "" || utf8.RuneCountInString(strings.TrimSpace(*dto.Name.Value)) > 100 {
			return domain.ErrInvalidUser
		}
	}
	if dto.Email.Set {
		if dto.Email.Value == nil || !validEmailSyntax(*dto.Email.Value) {
			return domain.ErrInvalidUser
		}
	}
	if err := validateContactDTO(dto.Phone.Value, 20); err != nil && dto.Phone.Set {
		return err
	}
	if err := validateContactDTO(dto.LineID.Value, 50); err != nil && dto.LineID.Set {
		return err
	}
	return nil
}

func validateContactDTO(value *string, maxRunes int) error {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || utf8.RuneCountInString(normalized) > maxRunes {
		return domain.ErrInvalidUser
	}
	return nil
}

func validEmailSyntax(value string) bool {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value && len(value) <= 255
}

func toUserResponse(user *domain.User) UserResponse {
	return UserResponse{
		UserID:    user.UserID,
		Name:      user.Name,
		Email:     user.Email,
		Phone:     user.Phone,
		LineID:    user.LineID,
		CreatedAt: user.CreatedAt,
	}
}

func validationError(err error) error {
	return response.NewAppError(fiber.StatusUnprocessableEntity, "validation_error", "request validation failed", err)
}

func mapUserError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return response.NewAppError(fiber.StatusNotFound, "user_not_found", "user not found", err)
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return response.NewAppError(fiber.StatusConflict, "email_already_exists", "email already exists", err)
	case errors.Is(err, domain.ErrUserForbidden):
		return response.NewAppError(fiber.StatusForbidden, "forbidden", "you do not have access to this user", err)
	case errors.Is(err, domain.ErrInvalidUser), errors.Is(err, domain.ErrNoUserChanges):
		return validationError(err)
	default:
		return err
	}
}
