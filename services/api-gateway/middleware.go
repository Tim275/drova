package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"drova/shared/env"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

var corsAllowedOrigin = env.GetString("CORS_ALLOWED_ORIGIN", "*")

func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", corsAllowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler(w, r)
	}
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://js.stripe.com https://api.mapbox.com https://events.mapbox.com; "+
				"style-src 'self' 'unsafe-inline' https://api.mapbox.com; "+
				"img-src 'self' data: blob: https://api.mapbox.com https://api.dicebear.com; "+
				"connect-src 'self' ws: wss: https://api.mapbox.com https://events.mapbox.com https://api.stripe.com; "+
				"frame-src https://js.stripe.com https://hooks.stripe.com; "+
				"worker-src blob:; "+
				"font-src 'self' data:;",
		)
		next.ServeHTTP(w, r)
	})
}

var slidingWindowScript = redis.NewScript(`
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count < limit then
  redis.call('ZADD', key, now, now .. ':' .. math.random(1,999999))
  redis.call('PEXPIRE', key, window)
  return 1
end
return 0
`)

func gatewayRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rdb == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			key := fmt.Sprintf("rl:gw:%s", ip)
			now := time.Now().UnixMilli()
			allowed, err := slidingWindowScript.Run(
				r.Context(), rdb,
				[]string{key},
				now, 1000, 30,
			).Int()
			if err != nil || allowed == 0 {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func newRedisClient(addr, password string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		appLog.Warnw("redis unavailable, rate limiting disabled", "err", err)
		return nil
	}
	return rdb
}
