package server

import (
	"time"

	"github.com/Aluminium51/cu-way-backend/internal/core/ports"
	httpapi "github.com/Aluminium51/cu-way-backend/internal/handlers/http"
	"github.com/Aluminium51/cu-way-backend/internal/middleware"
	"github.com/Aluminium51/cu-way-backend/internal/platform/response"
	"github.com/Aluminium51/cu-way-backend/internal/repositories/postgres"
	"github.com/Aluminium51/cu-way-backend/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Dependencies holds shared resources and ports required to build the HTTP server.
type Dependencies struct {
	Logger           zerolog.Logger
	DB               *gorm.DB
	ReadinessChecker ports.ReadinessChecker
	ReadinessTimeout time.Duration
	DocsPath         string
	TokenVerifier    ports.TokenVerifier
}

// New initializes, configures, and wires the Fiber application with middlewares and routes.
func New(deps Dependencies) *fiber.App {
	// Initialize Fiber
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          response.ErrorHandler(deps.Logger),
	})

	// global middleware pipeline
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(middleware.RequestLogger(deps.Logger)) // Logger

	// Serve the OpenAPI contract and the Scalar API Reference UI as public
	// documentation routes. DocsPath is injectable so tests do not depend on
	// the package working directory; production defaults to ./docs/openapi.yaml.
	docsPath := deps.DocsPath
	if docsPath == "" {
		docsPath = "./docs/openapi.yaml"
	}
	app.Static("/docs/openapi.yaml", docsPath)
	app.Get("/docs", func(c *fiber.Ctx) error {
		html := `<!doctype html>
<html>
  <head>
    <title>CU Ways API Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="/docs/openapi.yaml"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	})

	// [FUTURE] Configure CORS for Next.js frontend (e.g., localhost:3000)
	// app.Use(cors.New(cors.Config{
	// 	AllowOrigins: "http://localhost:3000",
	// 	AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	// }))

	// Wire dependencies and register system/health check endpoints
	healthHandler := httpapi.NewHealthHandler(
		services.NewHealthService(deps.ReadinessChecker, deps.ReadinessTimeout),
	)
	app.Get("/healthz", healthHandler.Health) // 200 = App is running
	app.Get("/readyz", healthHandler.Ready)   // 200 = Dependencies are healthy

	// Wire the first feature through the repository, service, and HTTP layers.
	userRepo := postgres.NewUserRepository(deps.DB)
	userService := services.NewUserService(userRepo)
	userHandler := httpapi.NewUserHandler(userService)
	api := app.Group("/api/v1")
	api.Post("/users", userHandler.Create)

	protectedUsers := middleware.RequireJWT(deps.TokenVerifier)
	api.Get("/users", protectedUsers, userHandler.List)
	api.Get("/users/:id", protectedUsers, userHandler.Get)
	api.Put("/users/:id", protectedUsers, userHandler.Update)
	api.Delete("/users/:id", protectedUsers, userHandler.Delete)

	// =========================================================================
	// 4. Dependency Injection & Repository Wiring [FUTURE]
	// =========================================================================
	// surveyRepo := postgres.NewSurveyRepository(deps.DB)
	// jobRepo := postgres.NewJobRepository(deps.DB)
	// paymentRepo := postgres.NewPaymentRepository(deps.DB)

	// =========================================================================
	// 5. Service Layer Instantiation [FUTURE]
	// =========================================================================
	// userService := services.NewUserService(userRepo)
	// surveyService := services.NewSurveyService(surveyRepo)
	// jobService := services.NewJobService(jobRepo, surveyRepo) // Handles state machines
	// paymentService := services.NewPaymentService(paymentRepo, jobRepo)

	// =========================================================================
	// 6. HTTP Handlers Instantiation [FUTURE]
	// =========================================================================
	// authHandler := httpapi.NewAuthHandler(userService, deps.TokenVerifier)
	// userHandler := httpapi.NewUserHandler(userService)
	// surveyHandler := httpapi.NewSurveyHandler(surveyService)
	// jobHandler := httpapi.NewJobHandler(jobService)
	// paymentHandler := httpapi.NewPaymentHandler(paymentService)

	// =========================================================================
	// 7. API Routes Registration [FUTURE]
	// =========================================================================
	// api := app.Group("/api/v1")

	// --- Public Routes ---
	// authGroup := api.Group("/auth")
	// authGroup.Post("/register", authHandler.Register)
	// authGroup.Post("/login", authHandler.Login)

	// --- Protected Routes (Require JWT Middleware) ---
	// protected := api.Group("", middleware.Auth(deps.TokenVerifier))
	//
	// User / Profile
	// protected.Get("/users/me", userHandler.GetProfile)
	// protected.Put("/users/me", userHandler.UpdateProfile)
	//
	// Survey Management
	// protected.Get("/surveys", surveyHandler.ListSurveys)
	// protected.Post("/surveys", surveyHandler.CreateSurvey)
	// protected.Get("/surveys/:id", surveyHandler.GetSurveyDetail)
	//
	// Job & Offer Flow (State Machine)
	// protected.Post("/jobs", jobHandler.CreateJob)
	// protected.Post("/jobs/:id/offers", jobHandler.SubmitOffer)
	// protected.Put("/jobs/:id/accept", jobHandler.AcceptJob)
	// protected.Put("/jobs/:id/complete", jobHandler.CompleteJob)
	//
	// Payment & Proof of Work
	// protected.Post("/jobs/:id/payments", paymentHandler.CreatePayment)
	// protected.Post("/jobs/:id/attachments", jobHandler.UploadProof)

	return app
}
