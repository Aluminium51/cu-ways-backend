package middleware

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/gofiber/fiber/v2"
)

const ClaimsLocalKey = "auth.claims"

func RequireJWT(verifier ports.TokenVerifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		parts := strings.Fields(strings.TrimSpace(c.Get(fiber.HeaderAuthorization)))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "authentication required", nil)
		}
		if verifier == nil {
			return response.NewAppError(fiber.StatusInternalServerError, "internal_error", "internal server error", nil)
		}

		claims, err := verifier.Verify(c.UserContext(), parts[1])
		if err != nil || claims == nil {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "invalid authentication token", err)
		}
		c.Locals(ClaimsLocalKey, claims)
		return c.Next()
	}
}

type ActiveUserLookup interface {
	FindByID(context.Context, int32) (*domain.User, error)
}

// RequireActiveUser checks account status and current permissions for every protected request.
func RequireActiveUser(users ActiveUserLookup) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := Claims(c)
		if !ok || claims == nil {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "authentication required", nil)
		}
		id, err := strconv.ParseInt(claims.Subject, 10, 32)
		if err != nil || id < 1 {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "invalid authentication token", err)
		}
		user, err := users.FindByID(c.UserContext(), int32(id))
		if errors.Is(err, domain.ErrUserNotFound) {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "account is inactive", err)
		}
		if err != nil {
			return err
		}
		// Copy claims before updating the role in case a verifier reuses its result.
		current := *claims
		current.Values = map[string]any{"role": user.Role}
		c.Locals(ClaimsLocalKey, &current)
		return c.Next()
	}
}

func Claims(c *fiber.Ctx) (*ports.TokenClaims, bool) {
	claims, ok := c.Locals(ClaimsLocalKey).(*ports.TokenClaims)
	return claims, ok
}
