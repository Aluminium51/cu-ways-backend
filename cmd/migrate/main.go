package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const migrationSource = "file://migrations"

func main() {
	if err := run(os.Getenv("DATABASE_URL"), migrationSource); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(databaseURL, source string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}

	m, err := migrate.New(source, databaseURL)
	if err != nil {
		return fmt.Errorf("initialize migrations failed (%T)", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		version, dirty, versionErr := m.Version()
		if versionErr != nil {
			return errors.New("apply migrations failed; migration version unavailable")
		}
		return fmt.Errorf("apply migrations failed at version %d (dirty=%t)", version, dirty)
	}
	fmt.Println("database migrations are current")
	return nil
}
