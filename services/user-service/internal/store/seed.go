package store

import (
	"context"

	"drova/services/user-service/internal/domain"
	"drova/shared/env"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type seedUser struct {
	email       string
	role        domain.Role
	displayName string
}

var seedUsers = []seedUser{
	{"rider@drova.local", domain.RoleRider, "Max Rider"},
	{"driver@drova.local", domain.RoleDriver, "Stefan Fahrer"},
}

// Seed creates (or rotates) the demo accounts. The password is taken from
// SEED_PASSWORD — production injects a strong value from a Sealed Secret, so the
// public "Test1234!" never works against prod. Dev/e2e leave it unset → the weak
// default, which keeps local + ephemeral-CI logins working.
//
// Existing users get their password ROTATED to the current SEED_PASSWORD. Without
// this, a prod account seeded once with the weak default would keep it forever.
func Seed(ctx context.Context, db *pgxpool.Pool, log *zap.SugaredLogger) {
	password := env.GetString("SEED_PASSWORD", "Test1234!")

	for _, u := range seedUsers {
		user := &domain.User{Email: u.email, Role: u.role}
		if err := user.Password.Set(password); err != nil {
			log.Warnw("seed: hash password failed", "email", u.email, zap.Error(err))
			continue
		}

		if _, err := db.Exec(ctx,
			`INSERT INTO users (email, password_hash, role, is_activated, display_name)
			 VALUES ($1, $2, $3, true, $4)
			 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
			u.email, user.Password.Hash(), u.role, u.displayName,
		); err != nil {
			log.Warnw("seed: upsert failed", "email", u.email, zap.Error(err))
			continue
		}
		log.Infow("seed: ensured user", "email", u.email, "role", u.role)
	}
}
