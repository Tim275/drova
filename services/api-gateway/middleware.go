package main

import (
	"context"
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

var corsAllowedOrigin = env.GetString("CORS_ALLOWED_ORIGIN", "")

func enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if corsAllowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", corsAllowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
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
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' blob: https://js.stripe.com https://api.mapbox.com https://events.mapbox.com https://m.stripe.network; "+
				"style-src 'self' 'unsafe-inline' https://api.mapbox.com https://fonts.googleapis.com; "+
				"img-src 'self' data: blob: https://api.mapbox.com https://api.dicebear.com; "+
				"connect-src 'self' ws: wss: https://api.mapbox.com https://events.mapbox.com https://api.stripe.com https://m.stripe.network; "+
				"frame-src https://js.stripe.com https://hooks.stripe.com; "+
				"worker-src blob:; "+
				"font-src 'self' data: https://fonts.googleapis.com https://fonts.gstatic.com;",
		)
		next.ServeHTTP(w, r)
	})
}

func newRedisClient(addr, password string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Don't discard the client on a startup ping failure — that permanently
		// disables Redis (rate limiting + WS tickets) until the pod restarts.
		// Common after a node reboot when Cilium egress policy hasn't converged
		// yet (connect: operation not permitted). go-redis reconnects lazily per
		// command, so keep the client and recover automatically once Redis is up.
		appLog.Warnw("redis ping failed at startup, will reconnect lazily", "err", err)
		return rdb
	}
	return rdb
}
