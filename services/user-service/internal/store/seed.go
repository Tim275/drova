package store

import (
	"context"

	"drova/services/user-service/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
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

func Seed(ctx context.Context, db *pgxpool.Pool, log *zap.SugaredLogger) {
	for _, u := range seedUsers {
		var exists bool
		db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", u.email).Scan(&exists)
		if exists {
			log.Infow("seed: already exists, skipping", "email", u.email)
			continue
		}

		user := &domain.User{Email: u.email, Role: u.role}
		if err := user.Password.Set(u.password); err != nil {
			log.Warnw("seed: hash password failed", "email", u.email, zap.Error(err))
			continue
		}

		if _, err := db.Exec(ctx,
			`INSERT INTO users (email, password_hash, role, is_activated, display_name) VALUES ($1, $2, $3, true, $4)`,
			u.email, user.Password.Hash(), u.role, u.displayName,
		); err != nil {
			log.Warnw("seed: insert failed", "email", u.email, zap.Error(err))
			continue
		}
		log.Infow("seed: created user", "email", u.email, "role", u.role)
	}
}
