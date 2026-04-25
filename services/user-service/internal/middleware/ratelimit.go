package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	mu       sync.Mutex
	limiters = make(map[string]*ipLimiter)
)

func init() {
	// Prune stale IP entries every minute to prevent unbounded growth (Factor 6).
	go func() {
		for range time.Tick(time.Minute) {
			mu.Lock()
			for ip, l := range limiters {
				if time.Since(l.lastSeen) > 3*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

func getLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	l, ok := limiters[ip]
	if !ok {
		l = &ipLimiter{limiter: rate.NewLimiter(rate.Limit(20), 40)}
		limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter
}

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !getLimiter(ip).Allow() {
			WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
