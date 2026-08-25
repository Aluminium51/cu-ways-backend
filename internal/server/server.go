package server

import (
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	httpapi "github.com/Aluminium51/cu-way-backend/internal/handlers/http"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
)

type Dependencies struct {
	Logger           zerolog.Logger
	ReadinessChecker ports.ReadinessChecker
	ReadinessTimeout time.Duration
}

func New(deps Dependencies) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          response.ErrorHandler(deps.Logger),
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(middleware.RequestLogger(deps.Logger))

	healthHandler := httpapi.NewHealthHandler(services.NewHealthService(deps.ReadinessChecker, deps.ReadinessTimeout))
	app.Get("/healthz", healthHandler.Health)
	app.Get("/readyz", healthHandler.Ready)

	return app
}
