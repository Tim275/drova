package grpc_clients

import (
	"context"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
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
	breakerMetricOnce.Do(registerBreakerMetric)
}

var breakerMetricOnce sync.Once

// Observable statt Counter beim Zustandswechsel: ein offener Breaker meldet sich
// so bei jedem Scrape, nicht nur im Moment des Umschaltens.
// Prometheus sieht: circuitbreaker_state (0=closed, 1=half-open, 2=open).
func registerBreakerMetric() {
	meter := otel.Meter("drova/circuitbreaker")
	_, err := meter.Int64ObservableGauge("circuitbreaker.state",
		metric.WithDescription("0=closed, 1=half-open, 2=open"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			for _, cb := range []*gobreaker.CircuitBreaker{TripBreaker, DriverBreaker, UserBreaker} {
				if cb == nil {
					continue
				}
				o.Observe(int64(cb.State()), metric.WithAttributes(attribute.String("target", cb.Name())))
			}
			return nil
		}))
	_ = err
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
