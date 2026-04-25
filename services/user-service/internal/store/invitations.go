package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InvitationStore struct {
	db *pgxpool.Pool
}

func NewInvitationStore(db *pgxpool.Pool) *InvitationStore {
	return &InvitationStore{db: db}
}

func (s *InvitationStore) Create(ctx context.Context, token string, userID int64, expiry time.Duration) error {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	_, err := s.db.Exec(ctx, `
		INSERT INTO user_invitations (token, user_id, expiry)
		VALUES ($1, $2, $3)`,
		tokenHash, userID, time.Now().Add(expiry),
	)
	return err
}

func (s *InvitationStore) Delete(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])
	_, err := s.db.Exec(ctx, `DELETE FROM user_invitations WHERE token = $1`, tokenHash)
	return err
}
