package grpc_clients

import (
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsBreakerError returns true only for infrastructure failures (timeouts, unavailable).
// Business errors like InvalidArgument or NotFound should not trip the breaker.
func IsBreakerError(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return true
	}
	switch st.Code() {
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists,
		codes.PermissionDenied, codes.Unauthenticated, codes.Canceled:
		return false
	}
	return true
}

var (
	TripBreaker   *gobreaker.CircuitBreaker
	DriverBreaker *gobreaker.CircuitBreaker
	UserBreaker   *gobreaker.CircuitBreaker
)

func InitBreakers(log *zap.SugaredLogger) {
	TripBreaker = newBreaker("trip-service", log)
	DriverBreaker = newBreaker("driver-service", log)
	UserBreaker = newBreaker("user-service", log)
}

func newBreaker(name string, log *zap.SugaredLogger) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 3,
		Interval:    30 * time.Second,
		Timeout:     5 * time.Second, // recover fast after short restarts
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 10 // high threshold for local dev
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Warnw("circuit breaker state changed", "service", name, "from", from, "to", to)
		},
	})
}
