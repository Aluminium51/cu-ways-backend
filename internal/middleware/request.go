package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func RequestLogger(logger zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		started := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		event := logger.Info()
		if status >= fiber.StatusInternalServerError {
			event = logger.Error()
		} else if status >= fiber.StatusBadRequest {
			event = logger.Warn()
		}
		event.
			Str("request_id", c.GetRespHeader(fiber.HeaderXRequestID)).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", status).
			Dur("duration", time.Since(started)).
			Msg("http request")
		return err
	}
}
