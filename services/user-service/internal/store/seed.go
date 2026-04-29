package store

import (
	"context"
	"log"

	"drova/services/user-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type seedUser struct {
	email       string
	password    string
	role        domain.Role
	displayName string
}

var seedUsers = []seedUser{
	{"rider@drova.local", "Test1234!", domain.RoleRider, "Max Rider"},
	{"driver@drova.local", "Test1234!", domain.RoleDriver, "Stefan Fahrer"},
}

func Seed(ctx context.Context, db *pgxpool.Pool) {
	for _, u := range seedUsers {
		var exists bool
		db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", u.email).Scan(&exists)
		if exists {
			log.Printf("seed: %s already exists — skipping", u.email)
			continue
		}

		user := &domain.User{Email: u.email, Role: u.role}
		if err := user.Password.Set(u.password); err != nil {
			log.Printf("seed: hash password for %s: %v", u.email, err)
			continue
		}

		if _, err := db.Exec(ctx,
			`INSERT INTO users (email, password_hash, role, is_activated, display_name) VALUES ($1, $2, $3, true, $4)`,
			u.email, user.Password.Hash(), u.role, u.displayName,
		); err != nil {
			log.Printf("seed: insert %s: %v", u.email, err)
			continue
		}
		log.Printf("seed: created %s (%s)", u.email, u.role)
	}
}
