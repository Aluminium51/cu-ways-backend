package database

import (
	"context"
	"database/sql"

	"github.com/Aluminium51/cu-way-backend/internal/config"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	gormDB *gorm.DB
	sqlDB  *sql.DB
}

func New(cfg config.DatabaseConfig, appLogger zerolog.Logger, environment string) (*DB, error) {
	gormLogger := logger.Silent
	if environment == "development" {
		gormLogger = logger.Warn
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{Logger: logger.Default.LogMode(gormLogger)})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	appLogger.Debug().Int("max_open_conns", cfg.MaxOpenConns).Int("max_idle_conns", cfg.MaxIdleConns).Msg("database configured")
	return &DB{gormDB: gormDB, sqlDB: sqlDB}, nil
}

func (db *DB) Ping(ctx context.Context) error {
	return db.sqlDB.PingContext(ctx)
}

func (db *DB) Close() error {
	return db.sqlDB.Close()
}

func (db *DB) GORM() *gorm.DB {
	return db.gormDB
}
