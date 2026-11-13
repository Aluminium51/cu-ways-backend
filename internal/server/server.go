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
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	// Serve the OpenAPI contract, Scalar, and Swagger UI as public
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
	app.Get("/docs/swagger", func(c *fiber.Ctx) error {
		const html = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>CU Ways API | Swagger UI</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.34.1/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.34.1/swagger-ui-bundle.js" crossorigin="anonymous"></script>
    <script>
      SwaggerUIBundle({
        url: "/docs/openapi.yaml",
        dom_id: "#swagger-ui",
        validatorUrl: null
      });
    </script>
  </body>
</html>`
		c.Set("Content-Type", "text/html; charset=utf-8")
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
	authLimit := limiter.New(limiter.Config{
		Max:        60,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return response.NewAppError(fiber.StatusTooManyRequests, "rate_limited", "Too many authentication requests. Try again shortly.", nil)
		},
	})
	auth.Post("/register", authLimit, authHandler.Register)
	auth.Post("/login", authLimit, authHandler.Login)

	protectedUsers := []fiber.Handler{middleware.RequireJWT(deps.TokenVerifier)}
	if deps.DB != nil {
		protectedUsers = append(protectedUsers, middleware.RequireActiveUser(userRepo))
	}
	protected := func(handler fiber.Handler) []fiber.Handler {
		return append(protectedUsers[:len(protectedUsers):len(protectedUsers)], handler)
	}
	api.Get("/users", protected(userHandler.List)...)
	api.Get("/users/:id", protected(userHandler.Get)...)
	api.Put("/users/:id", protected(userHandler.Update)...)
	api.Delete("/users/:id", protected(userHandler.Delete)...)

	adminReports := httpapi.NewAdminReportHandler(services.NewAdminReportService(postgres.NewAdminReportRepository(deps.DB)))
	api.Get("/admin/jobs", protected(adminReports.ListJobs)...)
	api.Get("/admin/flagged-marketers", protected(adminReports.ListFlaggedMarketers)...)

	// Profile, service, survey, and marketer search feature wiring.
	memberships := postgres.NewMembershipRepository(deps.DB)
	catalogRepo := postgres.NewCatalogRepository(deps.DB)
	marketerRepo := postgres.NewMarketerProfileRepository(deps.DB)
	marketerService := services.NewMarketerService(marketerRepo, catalogRepo, memberships)
	marketerHandler := httpapi.NewMarketerHandler(marketerService)
	reviewRepo := postgres.NewReviewRepository(deps.DB)
	marketerReviewService := services.NewMarketerReviewService(reviewRepo, marketerRepo, memberships)
	marketerReviewHandler := httpapi.NewMarketerReviewHandler(marketerReviewService)
	serviceRepo := postgres.NewServiceRepository(deps.DB)
	serviceService := services.NewServiceService(serviceRepo, memberships)
	serviceHandler := httpapi.NewServiceHandler(serviceService)
	statisticsRepo := postgres.NewMarketerStatisticsRepository(deps.DB)
	statisticsService := services.NewStatisticsService(statisticsRepo, memberships)
	statisticsHandler := httpapi.NewStatisticsHandler(statisticsService)
	surveyRepo := postgres.NewSurveyRepository(deps.DB)
	surveyService := services.NewSurveyService(surveyRepo)
	surveyHandler := httpapi.NewSurveyHandler(surveyService)

	api.Get("/marketers", protected(marketerHandler.Search)...)
	api.Get("/marketers/:id", protected(marketerHandler.GetDetail)...)
	api.Get("/marketers/:id/reviews", protected(marketerReviewHandler.List)...)
	api.Get("/me/marketer-profile", protected(marketerHandler.GetProfile)...)
	api.Patch("/me/marketer-profile", protected(marketerHandler.SaveProfile)...)
	api.Get("/me/services", protected(serviceHandler.List)...)
	api.Post("/me/services", protected(serviceHandler.Create)...)
	api.Patch("/me/services/:id", protected(serviceHandler.Update)...)
	api.Delete("/me/services/:id", protected(serviceHandler.Delete)...)
	api.Get("/me/statistics", protected(statisticsHandler.GetMyStatistics)...)
	api.Post("/surveys", protected(surveyHandler.Create)...)
	api.Get("/surveys/:id", protected(surveyHandler.Get)...)
	api.Patch("/surveys/:id", protected(surveyHandler.Update)...)
	api.Delete("/surveys/:id", protected(surveyHandler.Delete)...)
	jobRepo := postgres.NewJobRepository(deps.DB)
	jobService := services.NewJobService(jobRepo, memberships)
	jobHandler := httpapi.NewJobHandler(jobService)
	api.Get("/me/surveys", protected(jobHandler.ListSurveys)...)
	api.Get("/me/jobs", protected(jobHandler.ListMyJobs)...)
	api.Post("/jobs", protected(jobHandler.Create)...)
	api.Get("/jobs/:id", protected(jobHandler.GetJob)...)
	api.Patch("/jobs/:id", protected(jobHandler.UpdateDetails)...)
	api.Post("/jobs/:id/surveys/:surveyId", protected(jobHandler.AddSurvey)...)
	api.Delete("/jobs/:id/surveys/:surveyId", protected(jobHandler.RemoveSurvey)...)
	api.Post("/jobs/:id/briefs", protected(jobHandler.AddBrief)...)
	api.Get("/jobs/:id/briefs", protected(jobHandler.ListBriefs)...)
	api.Post("/jobs/:id/requests", protected(jobHandler.CreateRequests)...)
	api.Post("/jobs/:id/cancel", protected(jobHandler.Cancel)...)
	api.Get("/jobs/:id/requests", protected(jobHandler.ListJobRequests)...)
	marketerJobRequestRepo := postgres.NewMarketerJobRequestRepository(deps.DB)
	marketerJobRequestService := services.NewMarketerJobRequestService(marketerJobRequestRepo, memberships)
	marketerJobRequestHandler := httpapi.NewMarketerJobRequestHandler(marketerJobRequestService)
	api.Get("/me/job-requests", protected(marketerJobRequestHandler.List)...)
	api.Post("/me/job-requests/:id/accept", protected(marketerJobRequestHandler.Accept)...)
	api.Post("/me/job-requests/:id/decline", protected(marketerJobRequestHandler.Decline)...)
	api.Get("/me/job-requests/:id/offers", protected(marketerJobRequestHandler.ListOffers)...)
	api.Post("/me/job-requests/:id/offers", protected(marketerJobRequestHandler.SubmitOffer)...)
	api.Post("/me/offers/:id/withdraw", protected(marketerJobRequestHandler.WithdrawOffer)...)
	jobOfferRepo := postgres.NewJobOfferRepository(deps.DB)
	jobOfferService := services.NewJobOfferService(jobOfferRepo, memberships)
	jobOfferHandler := httpapi.NewJobOfferHandler(jobOfferService)
	api.Get("/jobs/:id/offers", protected(jobOfferHandler.List)...)
	api.Post("/jobs/:id/offers/:offerId/accept", protected(jobOfferHandler.Accept)...)
	marketerJobRepo := postgres.NewMarketerJobRepository(deps.DB)
	marketerJobService := services.NewMarketerJobService(marketerJobRepo, memberships)
	marketerJobHandler := httpapi.NewMarketerJobHandler(marketerJobService)
	api.Post("/jobs/:id/start", protected(marketerJobHandler.Start)...)
	paymentRepo := postgres.NewPaymentRepository(deps.DB)
	paymentService := services.NewPaymentService(paymentRepo, memberships)
	paymentHandler := httpapi.NewPaymentHandler(paymentService)
	api.Post("/jobs/:id/payments", protected(paymentHandler.Create)...)
	api.Get("/jobs/:id/payments/current", protected(paymentHandler.Current)...)
	proofRepo := postgres.NewProofRepository(deps.DB)
	proofService := services.NewProofService(proofRepo, memberships)
	proofHandler := httpapi.NewProofHandler(proofService)
	api.Post("/jobs/:id/proofs", protected(proofHandler.Create)...)
	api.Get("/jobs/:id/proofs", protected(proofHandler.List)...)
	reviewService := services.NewReviewService(reviewRepo, memberships)
	reviewHandler := httpapi.NewReviewHandler(reviewService)
	api.Post("/jobs/:id/reviews", protected(reviewHandler.Submit)...)
	api.Get("/jobs/:id/reviews/current", protected(reviewHandler.Current)...)
	settlementRepo := postgres.NewJobSettlementRepository(deps.DB)
	settlementHandler := httpapi.NewJobSettlementHandler(services.NewJobSettlementService(settlementRepo, memberships))
	api.Get("/jobs/:id/payments", protected(settlementHandler.GetPayment)...)
	api.Post("/jobs/:id/complete", protected(settlementHandler.Complete)...)
	transactionRepo := postgres.NewTransactionRepository(deps.DB)
	transactionService := services.NewTransactionService(transactionRepo)
	transactionHandler := httpapi.NewTransactionHandler(transactionService)
	api.Get("/me/transactions", protected(transactionHandler.List)...)

	return app
}
