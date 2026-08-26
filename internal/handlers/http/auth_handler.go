package httpapi

import (
	"context"
	"errors"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type authService interface {
	Register(context.Context, services.RegisterInput) (*services.AuthResult, error)
	Login(context.Context, services.LoginInput) (*services.AuthResult, error)
}

type AuthHandler struct {
	service authService
}

func NewAuthHandler(service authService) *AuthHandler {
	return &AuthHandler{service: service}
}

type RegisterDTO struct {
	Name     string  `json:"name" validate:"required,max=100"`
	Email    string  `json:"email" validate:"required,email,max=255"`
	Password string  `json:"password" validate:"required,min=8,max=128"`
	Phone    *string `json:"phone" validate:"omitempty,max=20"`
	LineID   *string `json:"line_id" validate:"omitempty,max=50"`
}

type LoginDTO struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var dto RegisterDTO
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

	result, err := h.service.Register(c.UserContext(), services.RegisterInput{
		Name:     dto.Name,
		Email:    dto.Email,
		Password: dto.Password,
		Phone:    dto.Phone,
		LineID:   dto.LineID,
	})
	if err != nil {
		return mapAuthError(err)
	}
	return response.Success(c, fiber.StatusCreated, toAuthResponse(result))
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var dto LoginDTO
	if err := c.BodyParser(&dto); err != nil {
		return validationError(err)
	}
	if err := utils.Validate(dto); err != nil {
		return validationError(err)
	}

	result, err := h.service.Login(c.UserContext(), services.LoginInput{
		Email:    dto.Email,
		Password: dto.Password,
	})
	if err != nil {
		return mapAuthError(err)
	}
	return response.Success(c, fiber.StatusOK, toAuthResponse(result))
}

func toAuthResponse(result *services.AuthResult) AuthResponse {
	return AuthResponse{
		AccessToken: result.Token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(services.AccessTokenTTL.Seconds()),
		User:        toUserResponse(result.User),
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return response.NewAppError(fiber.StatusUnauthorized, "invalid_credentials", "invalid email or password", nil)
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return response.NewAppError(fiber.StatusConflict, "email_already_exists", "email already exists", err)
	case errors.Is(err, domain.ErrInvalidUser):
		return validationError(err)
	default:
		return err
	}
}
