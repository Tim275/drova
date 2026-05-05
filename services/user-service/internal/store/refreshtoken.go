package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const refreshPrefix = "drova:refresh:"

type refreshPayload struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}

type RefreshTokenStore struct {
	rdb *redis.Client
}

func NewRefreshTokenStore(rdb *redis.Client) *RefreshTokenStore {
	return &RefreshTokenStore{rdb: rdb}
}

func (s *RefreshTokenStore) Create(ctx context.Context, token string, userID int64, role string, ttl time.Duration) error {
	payload, err := json.Marshal(refreshPayload{UserID: userID, Role: role})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, refreshPrefix+token, payload, ttl).Err()
}

func (s *RefreshTokenStore) Validate(ctx context.Context, token string) (int64, string, error) {
	raw, err := s.rdb.Get(ctx, refreshPrefix+token).Result()
	if errors.Is(err, redis.Nil) {
		return 0, "", fmt.Errorf("refresh token not found or expired")
	}
	if err != nil {
		return 0, "", err
	}
	var p refreshPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return 0, "", err
	}
	return p.UserID, p.Role, nil
}

func (s *RefreshTokenStore) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, refreshPrefix+token).Err()
}
