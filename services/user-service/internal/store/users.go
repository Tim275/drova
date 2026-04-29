package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"drova/services/user-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore struct {
	db *pgxpool.Pool
}

func NewUserStore(db *pgxpool.Pool) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(ctx context.Context, user *domain.User) error {
	return withTx(ctx, s.db, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO users (email, password_hash, role, display_name, phone)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at`,
			user.Email, user.Password.Hash(), user.Role, user.DisplayName, user.Phone,
		).Scan(&user.ID, &user.CreatedAt)
	})
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	var hash []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, is_activated, created_at,
		       COALESCE(display_name,''), COALESCE(avatar_url,''), COALESCE(phone,''), COALESCE(address,'')
		FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &hash, &u.Role, &u.IsActivated, &u.CreatedAt,
		&u.DisplayName, &u.AvatarURL, &u.Phone, &u.Address)
	if err != nil {
		return nil, err
	}
	u.Password.SetHash(hash)
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	var hash []byte
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, role, is_activated, created_at,
		       COALESCE(display_name,''), COALESCE(avatar_url,''), COALESCE(phone,''), COALESCE(address,'')
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &hash, &u.Role, &u.IsActivated, &u.CreatedAt,
		&u.DisplayName, &u.AvatarURL, &u.Phone, &u.Address)
	if err != nil {
		return nil, err
	}
	u.Password.SetHash(hash)
	return &u, nil
}

func (s *UserStore) UpdateProfile(ctx context.Context, userID int64, displayName, avatarURL, phone, address string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE users SET display_name=$1, avatar_url=$2, phone=$3, address=$4
		WHERE id=$5`,
		displayName, avatarURL, phone, address, userID)
	return err
}

func (s *UserStore) Activate(ctx context.Context, token string) (*domain.User, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var u domain.User
	err := s.db.QueryRow(ctx, `
		UPDATE users SET is_activated = true
		WHERE id = (
			SELECT user_id FROM user_invitations
			WHERE token = $1 AND expiry > $2
		)
		RETURNING id, email, role, is_activated, created_at`,
		tokenHash, time.Now(),
	).Scan(&u.ID, &u.Email, &u.Role, &u.IsActivated, &u.CreatedAt)
	return &u, err
}
