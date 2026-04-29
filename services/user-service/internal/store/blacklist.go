package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const blacklistPrefix = "jti:revoked:"

type TokenBlacklist struct {
	rdb *redis.Client
}

func NewTokenBlacklist(rdb *redis.Client) *TokenBlacklist {
	return &TokenBlacklist{rdb: rdb}
}

func (b *TokenBlacklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return b.rdb.Set(ctx, blacklistPrefix+jti, "1", ttl).Err()
}

func (b *TokenBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := b.rdb.Exists(ctx, blacklistPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
