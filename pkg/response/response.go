package response

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type SuccessEnvelope struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
}

type ErrorEnvelope struct {
	Status string   `json:"status"`
	Error  APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type AppError struct {
	Status  int
	Code    string
	Message string
	Details any
	Err     error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(status int, code, message string, err error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Err: err}
}

func Success(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(SuccessEnvelope{Status: "success", Data: data})
}

func ErrorHandler(logger zerolog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		status := fiber.StatusInternalServerError
		code := "internal_error"
		message := "internal server error"
		var details any

		var appErr *AppError
		if errors.As(err, &appErr) {
			status = appErr.Status
			code = appErr.Code
			message = appErr.Message
			details = appErr.Details
		} else {
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
				code = fiberCode(status)
				message = fiberMessage(status, fiberErr.Message)
			}
		}

		if status >= fiber.StatusInternalServerError {
			logger.Error().Err(err).Str("request_id", c.GetRespHeader(fiber.HeaderXRequestID)).Msg("request failed")
		} else {
			logger.Warn().Err(err).Str("request_id", c.GetRespHeader(fiber.HeaderXRequestID)).Int("status", status).Msg("request rejected")
		}

		return c.Status(status).JSON(ErrorEnvelope{
			Status: "error",
			Error:  APIError{Code: code, Message: message, Details: details},
		})
	}
}

func fiberCode(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusBadRequest:
		return "bad_request"
	default:
		return "http_error"
	}
}

func fiberMessage(status int, fallback string) string {
	if status >= fiber.StatusInternalServerError {
		return "internal server error"
	}
	return fallback
}
