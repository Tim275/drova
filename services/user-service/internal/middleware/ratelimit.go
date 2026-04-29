package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

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

func RateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	const (
		windowMs = 1000
		limit    = 20
	)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			now := time.Now().UnixMilli()
			key := "rl:user-svc:" + ip

			allowed, err := slidingWindowScript.Run(r.Context(), rdb, []string{key},
				now, windowMs, limit,
			).Int()
			if err != nil || allowed == 1 {
				next.ServeHTTP(w, r)
				return
			}
			WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
		})
	}
}
