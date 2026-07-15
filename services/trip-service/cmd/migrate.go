package main

import (
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(dbURL, migrationsPath string) error {
	stripped := strings.TrimPrefix(dbURL, "postgresql://")
	stripped = strings.TrimPrefix(stripped, "postgres://")
	migrateURL := "pgx5://" + stripped

	// Retry transient DB unavailability (e.g. Postgres failover) before failing fast.
	const maxAttempts = 8
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = migrateOnce(migrationsPath, migrateURL); err == nil {
			return nil
		}
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}
	return err
}

func migrateOnce(migrationsPath, migrateURL string) error {
	m, err := migrate.New("file://"+migrationsPath, migrateURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
