package main

import (
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(dbURL, migrationsPath string) error {
	migrateURL := "pgx5://" + strings.TrimPrefix(dbURL, "postgres://")
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
