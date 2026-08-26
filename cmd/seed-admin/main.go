package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/Aluminium51/cu-way-backend/internal/core/domain"
	"github.com/Aluminium51/cu-way-backend/internal/platform/database"
	"github.com/Aluminium51/cu-way-backend/internal/platform/logging"
	"github.com/Aluminium51/cu-way-backend/internal/platform/utils"
	"github.com/Aluminium51/cu-way-backend/internal/repositories/postgres"
	"github.com/Aluminium51/cu-way-backend/internal/services"
)

const defaultAdminName = "CU Ways Admin"

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
	if err := validateSeedEnvironment(cfg.App.Environment); err != nil {
		return err
	}

	input, err := seedAdminInput(cfg.SeedAdmin)
	if err != nil {
		return err
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

	seedService := services.NewAdminSeedService(
		postgres.NewUserRepository(db.GORM()),
		utils.NewArgon2idPasswordHasher(),
	)
	result, err := seedService.Seed(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAdminSeedUserDeleted):
			return fmt.Errorf("seed admin: account for %s is soft-deleted; choose another email", input.Email)
		case errors.Is(err, services.ErrAdminSeedPasswordless):
			return fmt.Errorf("seed admin: account for %s has no password; seed does not overwrite existing credentials", input.Email)
		case errors.Is(err, domain.ErrInvalidUser):
			return fmt.Errorf("seed admin: invalid name, email, or password")
		default:
			return fmt.Errorf("seed admin: %w", err)
		}
	}

	if result.Created {
		fmt.Printf("created admin account for %s\n", result.User.Email)
	} else {
		fmt.Printf("admin account ready for %s; existing password preserved\n", result.User.Email)
	}
	return nil
}

func validateSeedEnvironment(environment string) error {
	if environment != "development" && environment != "test" {
		return fmt.Errorf("seed admin is only allowed in development or test environment, got %q", environment)
	}
	return nil
}

func seedAdminInput(cfg config.SeedAdminConfig) (services.SeedAdminInput, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = defaultAdminName
	}
	if strings.TrimSpace(cfg.Email) == "" {
		return services.SeedAdminInput{}, errors.New("SEED_ADMIN_EMAIL is required")
	}
	if cfg.Password == "" {
		return services.SeedAdminInput{}, errors.New("SEED_ADMIN_PASSWORD is required")
	}
	return services.SeedAdminInput{Name: name, Email: cfg.Email, Password: cfg.Password}, nil
}
