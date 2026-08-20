package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/Aluminium51/cu-way-backend/internal/logging"
	"github.com/Aluminium51/cu-way-backend/internal/server"
	"github.com/Aluminium51/cu-way-backend/pkg/database"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

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

	app := server.New(server.Dependencies{
		Logger:           appLogger,
		ReadinessChecker: db,
		ReadinessTimeout: cfg.Server.ReadinessTimeout,
	})

	listenErrors := make(chan error, 1)
	go func() {
		listenErrors <- app.Listen(fmt.Sprintf(":%d", cfg.Server.Port))
	}()

	select {
	case err := <-listenErrors:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		shutdownErrors := make(chan error, 1)
		go func() {
			shutdownErrors <- app.Shutdown()
		}()
		select {
		case err := <-shutdownErrors:
			if err != nil {
				return fmt.Errorf("shutdown server: %w", err)
			}
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timeout: %w", shutdownCtx.Err())
		}
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
