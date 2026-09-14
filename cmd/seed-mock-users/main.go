package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/Aluminium51/cu-way-backend/internal/platform/database"
	"github.com/Aluminium51/cu-way-backend/internal/platform/logging"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/repositories/postgres"
	"github.com/Aluminium51/cu-way-backend/internal/services"
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
	if err := validateMockDataEnvironment(cfg.App.Environment); err != nil {
		return err
	}
	if cfg.MockData.UserPassword == "" {
		return errors.New("MOCK_USER_PASSWORD is required")
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

	seeder := services.NewMockDataSeeder(
		postgres.NewMockDataRepository(db.GORM()),
		utils.NewArgon2idPasswordHasher(),
	)
	summary, err := seeder.Seed(ctx, cfg.MockData.UserPassword)
	if err != nil {
		return fmt.Errorf("seed mock users: %w", err)
	}

	fmt.Printf("seeded %d mock users (%d creators, %d marketers, %d both)\n", summary.Users, summary.Creators, summary.Marketers, summary.Both)
	return nil
}

func validateMockDataEnvironment(environment string) error {
	if environment != "development" && environment != "test" {
		return fmt.Errorf("mock data seeding is only allowed in development or test environment, got %q", environment)
	}
	return nil
}
