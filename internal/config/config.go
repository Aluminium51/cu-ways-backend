package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const JWTAlgorithm = "HS256"

type Config struct {
	App       AppConfig
	Server    ServerConfig
	Database  DatabaseConfig
	Auth      AuthConfig
	SeedAdmin SeedAdminConfig
}

type AppConfig struct {
	Environment string
}

type ServerConfig struct {
	Port             int
	ShutdownTimeout  time.Duration
	ReadinessTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type AuthConfig struct {
	SecretKey string
	Algorithm string
}

type SeedAdminConfig struct {
	Name     string
	Email    string
	Password string
}

func Load() (Config, error) {
	return LoadFromFile(".env")
}

func LoadFromFile(path string) (Config, error) {
	v := viper.New()
	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
		if _, err := os.Stat(path); err == nil {
			if err := v.ReadInConfig(); err != nil {
				return Config{}, fmt.Errorf("read config file: %w", err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("stat config file: %w", err)
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	return fromViper(v)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("PORT", 8081)
	v.SetDefault("SHUTDOWN_TIMEOUT", 10*time.Second)
	v.SetDefault("READINESS_TIMEOUT", 2*time.Second)
	v.SetDefault("DB_MAX_OPEN_CONNS", 10)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)
	v.SetDefault("DB_CONN_MAX_LIFETIME", time.Hour)
	v.SetDefault("DB_CONN_MAX_IDLE_TIME", 30*time.Minute)
}

func fromViper(v *viper.Viper) (Config, error) {
	appEnvironment := strings.ToLower(strings.TrimSpace(v.GetString("APP_ENV")))
	if appEnvironment == "" {
		return Config{}, errors.New("APP_ENV must not be empty")
	}

	port := v.GetInt("PORT")
	if port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535, got %d", port)
	}

	databaseURL := strings.TrimSpace(v.GetString("DATABASE_URL"))
	if err := validateDatabaseURL(databaseURL); err != nil {
		return Config{}, err
	}

	secretKey := strings.TrimSpace(v.GetString("SECRET_KEY"))
	if secretKey == "" {
		return Config{}, errors.New("SECRET_KEY is required")
	}
	if appEnvironment == "production" && len(secretKey) < 32 {
		return Config{}, errors.New("SECRET_KEY must be at least 32 characters in production")
	}

	maxOpen := v.GetInt("DB_MAX_OPEN_CONNS")
	maxIdle := v.GetInt("DB_MAX_IDLE_CONNS")
	if maxOpen < 1 || maxIdle < 0 || maxIdle > maxOpen {
		return Config{}, fmt.Errorf("invalid database pool settings: max open %d, max idle %d", maxOpen, maxIdle)
	}

	shutdownTimeout := v.GetDuration("SHUTDOWN_TIMEOUT")
	readinessTimeout := v.GetDuration("READINESS_TIMEOUT")
	if shutdownTimeout <= 0 || readinessTimeout <= 0 {
		return Config{}, errors.New("SHUTDOWN_TIMEOUT and READINESS_TIMEOUT must be positive")
	}

	return Config{
		App: AppConfig{Environment: appEnvironment},
		Server: ServerConfig{
			Port:             port,
			ShutdownTimeout:  shutdownTimeout,
			ReadinessTimeout: readinessTimeout,
		},
		Database: DatabaseConfig{
			URL:             databaseURL,
			MaxOpenConns:    maxOpen,
			MaxIdleConns:    maxIdle,
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME"),
			ConnMaxIdleTime: v.GetDuration("DB_CONN_MAX_IDLE_TIME"),
		},
		Auth: AuthConfig{
			SecretKey: secretKey,
			Algorithm: JWTAlgorithm,
		},
		SeedAdmin: SeedAdminConfig{
			Name:     strings.TrimSpace(v.GetString("SEED_ADMIN_NAME")),
			Email:    strings.TrimSpace(v.GetString("SEED_ADMIN_EMAIL")),
			Password: v.GetString("SEED_ADMIN_PASSWORD"),
		},
	}, nil
}

func validateDatabaseURL(value string) error {
	if value == "" {
		return errors.New("DATABASE_URL is required")
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("DATABASE_URL must be a valid postgres:// or postgresql:// URL")
	}
	return nil
}
