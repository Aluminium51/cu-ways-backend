package middleware

import (
	"strings"

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

		claims, err := verifier.Verify(c.UserContext(), parts[1])
		if err != nil {
			return response.NewAppError(fiber.StatusUnauthorized, "unauthorized", "invalid authentication token", err)
		}
		c.Locals(ClaimsLocalKey, claims)
		return c.Next()
	}
}

func Claims(c *fiber.Ctx) (*ports.TokenClaims, bool) {
	claims, ok := c.Locals(ClaimsLocalKey).(*ports.TokenClaims)
	return claims, ok
}
