package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"drova/services/user-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

const userCacheTTL = 1 * time.Hour

type UserCache struct {
	rdb *redis.Client
}

func NewUserCache(rdb *redis.Client) *UserCache {
	return &UserCache{rdb: rdb}
}

type cachedUser struct {
	ID          int64       `json:"id"`
	Email       string      `json:"email"`
	Role        domain.Role `json:"role"`
	IsActivated bool        `json:"is_activated"`
	CreatedAt   time.Time   `json:"created_at"`
}

func (c *UserCache) Get(ctx context.Context, id int64) (*domain.User, error) {
	if c.rdb == nil {
		return nil, nil
	}
	data, err := c.rdb.Get(ctx, userKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // cache miss is not an error
		}
		return nil, err
	}
	var cu cachedUser
	if err := json.Unmarshal(data, &cu); err != nil {
		return nil, err
	}
	return &domain.User{
		ID:          cu.ID,
		Email:       cu.Email,
		Role:        cu.Role,
		IsActivated: cu.IsActivated,
		CreatedAt:   cu.CreatedAt,
	}, nil
}

func (c *UserCache) Set(ctx context.Context, u *domain.User) error {
	if c.rdb == nil {
		return nil
	}
	cu := cachedUser{
		ID:          u.ID,
		Email:       u.Email,
		Role:        u.Role,
		IsActivated: u.IsActivated,
		CreatedAt:   u.CreatedAt,
	}
	data, err := json.Marshal(cu)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, userKey(u.ID), data, userCacheTTL).Err()
}

func (c *UserCache) Delete(ctx context.Context, id int64) {
	if c.rdb == nil {
		return
	}
	c.rdb.Del(ctx, userKey(id))
}

func userKey(id int64) string {
	return fmt.Sprintf("user:%d", id)
}
