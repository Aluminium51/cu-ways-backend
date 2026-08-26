package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/Aluminium51/cu-way-backend/internal/platform/database"
	"github.com/Aluminium51/cu-way-backend/internal/platform/logging"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/server"
)

func main() {
	// Listen for OS termination signals (SIGINT, SIGTERM) to initiate graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run application lifecycle and exit with status code 1 on unhandled errors
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Load and validate ENV configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// Logger and Database connection pool
	appLogger := logging.New(cfg.App.Environment)
	db, err := database.New(cfg.Database, appLogger, cfg.App.Environment)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error().Err(closeErr).Msg("close database")
		}
	}()

	// Compose Fiber app and inject required dependencies
	app := server.New(server.Dependencies{
		Logger:           appLogger,
		DB:               db.GORM(),
		ReadinessChecker: db,
		ReadinessTimeout: cfg.Server.ReadinessTimeout,
		TokenVerifier:    utils.NewConfiguredJWTVerifier(cfg.Auth),
		PasswordHasher:   utils.NewArgon2idPasswordHasher(),
		TokenIssuer:      utils.NewConfiguredJWTIssuer(cfg.Auth),
	})

	// Start HTTP server
	listenErrors := make(chan error, 1)
	go func() {
		listenErrors <- app.Listen(fmt.Sprintf(":%d", cfg.Server.Port))
	}()

	// Block until an unexpected server error occurs
	select {
	case err := <-listenErrors:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Create a timeout context to bound the shutdown window
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		// Request Fiber to stop accepting new requests and complete in-flight requests
		shutdownErrors := make(chan error, 1)
		go func() {
			shutdownErrors <- app.Shutdown()
		}()

		// Wait for Fiber shutdown to complete or timeout
		select {
		case err := <-shutdownErrors:
			if err != nil {
				return fmt.Errorf("shutdown server: %w", err)
			}
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timeout: %w", shutdownCtx.Err())
		}

		// Ensure the background listener goroutine has stopped cleanly
		select {
		case err := <-listenErrors:
			if err != nil {
				return fmt.Errorf("stop server: %w", err)
			}
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timeout: %w", shutdownCtx.Err())
		}
		return nil
	}
}
