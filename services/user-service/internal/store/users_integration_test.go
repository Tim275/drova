//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"

	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/store"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		t.Skip("TEST_DB_URL not set — skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)

	// Run migrations so the schema is always up-to-date.
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "../../../../services/user-service/migrations"
	}
	// Strip scheme for migrate URL
	migrateURL := "pgx5://" + dsn
	if len(dsn) > 13 && dsn[:13] == "postgresql://" {
		migrateURL = "pgx5://" + dsn[13:]
	} else if len(dsn) > 11 && dsn[:11] == "postgres://" {
		migrateURL = "pgx5://" + dsn[11:]
	}

	m, err := migrate.New("file://"+migrationsPath, migrateURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("run migrations: %v", err)
	}

	// Clean state before each test run.
	pool.Exec(context.Background(), "DELETE FROM user_invitations") //nolint:errcheck
	pool.Exec(context.Background(), "DELETE FROM users")            //nolint:errcheck

	return pool
}

func newUser(email string) *domain.User {
	u := &domain.User{
		Email:       email,
		Role:        domain.RoleRider,
		DisplayName: "Test Rider",
		Phone:       "+4915112345678",
	}
	if err := u.Password.Set("TestPass1!"); err != nil {
		panic(err)
	}
	return u
}

// After Create the user is retrievable by email with correct fields.
func TestUserStore_Create_And_GetByEmail(t *testing.T) {
	db := testDB(t)
	s := store.NewUserStore(db)
	ctx := context.Background()

	u := newUser("integration@drova.test")
	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Error("Create must populate user.ID")
	}

	got, err := s.GetByEmail(ctx, "integration@drova.test")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.Email != u.Email {
		t.Errorf("email mismatch: want %q got %q", u.Email, got.Email)
	}
	if got.DisplayName != u.DisplayName {
		t.Errorf("display_name mismatch: want %q got %q", u.DisplayName, got.DisplayName)
	}
	if got.IsActivated {
		t.Error("new user must not be activated")
	}
}

// Creating two users with the same email must return an error (unique constraint).
func TestUserStore_Create_DuplicateEmail(t *testing.T) {
	db := testDB(t)
	s := store.NewUserStore(db)
	ctx := context.Background()

	if err := s.Create(ctx, newUser("dup@drova.test")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(ctx, newUser("dup@drova.test")); err == nil {
		t.Error("second Create with same email must return an error")
	}
}

// GetByEmail for a non-existent address must return an error (not a nil user).
func TestUserStore_GetByEmail_NotFound(t *testing.T) {
	db := testDB(t)
	s := store.NewUserStore(db)

	_, err := s.GetByEmail(context.Background(), "nobody@drova.test")
	if err == nil {
		t.Error("GetByEmail must return an error for unknown email")
	}
}
