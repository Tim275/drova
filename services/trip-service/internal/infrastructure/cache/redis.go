package cache

import (
	"context"
	"encoding/json"
	"time"

	tripTypes "drova/services/trip-service/pkg/types"

	"github.com/redis/go-redis/v9"
)

const routeTTL = 5 * time.Minute

type RedisRouteCache struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *RedisRouteCache {
	return &RedisRouteCache{rdb: rdb}
}

func (c *RedisRouteCache) Get(ctx context.Context, key string) (*tripTypes.MapboxRouteResponse, bool) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var r tripTypes.MapboxRouteResponse
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, false
	}
	return &r, true
}

func (c *RedisRouteCache) Set(ctx context.Context, key string, route *tripTypes.MapboxRouteResponse) {
	b, err := json.Marshal(route)
	if err != nil {
		return
	}
	// fire-and-forget — cache failure is non-fatal
	c.rdb.Set(ctx, key, b, routeTTL) //nolint:errcheck
}
