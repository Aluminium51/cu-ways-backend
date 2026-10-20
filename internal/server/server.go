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
	PasswordHasher   ports.PasswordHasher
	TokenIssuer      ports.TokenIssuer
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
	authService := services.NewAuthService(userRepo, deps.PasswordHasher, deps.TokenIssuer)
	authHandler := httpapi.NewAuthHandler(authService)
	api := app.Group("/api/v1")
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)

	protectedUsers := middleware.RequireJWT(deps.TokenVerifier)
	api.Get("/users", protectedUsers, userHandler.List)
	api.Get("/users/:id", protectedUsers, userHandler.Get)
	api.Put("/users/:id", protectedUsers, userHandler.Update)
	api.Delete("/users/:id", protectedUsers, userHandler.Delete)

	// Profile, service, survey, and marketer search feature wiring.
	memberships := postgres.NewMembershipRepository(deps.DB)
	catalogRepo := postgres.NewCatalogRepository(deps.DB)
	marketerRepo := postgres.NewMarketerProfileRepository(deps.DB)
	marketerService := services.NewMarketerService(marketerRepo, catalogRepo, memberships)
	marketerHandler := httpapi.NewMarketerHandler(marketerService)
	serviceRepo := postgres.NewServiceRepository(deps.DB)
	serviceService := services.NewServiceService(serviceRepo, memberships)
	serviceHandler := httpapi.NewServiceHandler(serviceService)
	statisticsRepo := postgres.NewMarketerStatisticsRepository(deps.DB)
	statisticsService := services.NewStatisticsService(statisticsRepo, memberships)
	statisticsHandler := httpapi.NewStatisticsHandler(statisticsService)
	surveyRepo := postgres.NewSurveyRepository(deps.DB)
	surveyService := services.NewSurveyService(surveyRepo)
	surveyHandler := httpapi.NewSurveyHandler(surveyService)

	api.Get("/marketers", protectedUsers, marketerHandler.Search)
	api.Get("/marketers/:id", protectedUsers, marketerHandler.GetDetail)
	api.Get("/me/marketer-profile", protectedUsers, marketerHandler.GetProfile)
	api.Patch("/me/marketer-profile", protectedUsers, marketerHandler.SaveProfile)
	api.Get("/me/services", protectedUsers, serviceHandler.List)
	api.Post("/me/services", protectedUsers, serviceHandler.Create)
	api.Patch("/me/services/:id", protectedUsers, serviceHandler.Update)
	api.Delete("/me/services/:id", protectedUsers, serviceHandler.Delete)
	api.Get("/me/statistics", protectedUsers, statisticsHandler.GetMyStatistics)
	api.Post("/surveys", protectedUsers, surveyHandler.Create)
	api.Get("/surveys/:id", protectedUsers, surveyHandler.Get)
	api.Patch("/surveys/:id", protectedUsers, surveyHandler.Update)
	api.Delete("/surveys/:id", protectedUsers, surveyHandler.Delete)
	jobRepo := postgres.NewJobRepository(deps.DB)
	jobService := services.NewJobService(jobRepo, memberships)
	jobHandler := httpapi.NewJobHandler(jobService)
	api.Get("/me/surveys", protectedUsers, jobHandler.ListSurveys)
	api.Get("/me/jobs", protectedUsers, jobHandler.ListMyJobs)
	api.Post("/jobs", protectedUsers, jobHandler.Create)
	api.Get("/jobs/:id", protectedUsers, jobHandler.GetJob)
	api.Patch("/jobs/:id", protectedUsers, jobHandler.UpdateDetails)
	api.Post("/jobs/:id/surveys/:surveyId", protectedUsers, jobHandler.AddSurvey)
	api.Delete("/jobs/:id/surveys/:surveyId", protectedUsers, jobHandler.RemoveSurvey)
	api.Post("/jobs/:id/briefs", protectedUsers, jobHandler.AddBrief)
	api.Get("/jobs/:id/briefs", protectedUsers, jobHandler.ListBriefs)
	api.Post("/jobs/:id/requests", protectedUsers, jobHandler.CreateRequests)
	api.Get("/jobs/:id/requests", protectedUsers, jobHandler.ListJobRequests)
	marketerJobRequestRepo := postgres.NewMarketerJobRequestRepository(deps.DB)
	marketerJobRequestService := services.NewMarketerJobRequestService(marketerJobRequestRepo, memberships)
	marketerJobRequestHandler := httpapi.NewMarketerJobRequestHandler(marketerJobRequestService)
	api.Get("/me/job-requests", protectedUsers, marketerJobRequestHandler.List)
	api.Post("/me/job-requests/:id/accept", protectedUsers, marketerJobRequestHandler.Accept)
	api.Post("/me/job-requests/:id/decline", protectedUsers, marketerJobRequestHandler.Decline)
	api.Get("/me/job-requests/:id/offers", protectedUsers, marketerJobRequestHandler.ListOffers)
	api.Post("/me/job-requests/:id/offers", protectedUsers, marketerJobRequestHandler.SubmitOffer)
	api.Post("/me/offers/:id/withdraw", protectedUsers, marketerJobRequestHandler.WithdrawOffer)
	jobOfferRepo := postgres.NewJobOfferRepository(deps.DB)
	jobOfferService := services.NewJobOfferService(jobOfferRepo, memberships)
	jobOfferHandler := httpapi.NewJobOfferHandler(jobOfferService)
	api.Get("/jobs/:id/offers", protectedUsers, jobOfferHandler.List)
	api.Post("/jobs/:id/offers/:offerId/accept", protectedUsers, jobOfferHandler.Accept)

	return app
}
