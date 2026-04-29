package grpc_clients

import (
	"log"
	"time"

	"github.com/sony/gobreaker"
)

func newBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 2,
		Interval:    30 * time.Second,
		Timeout:     15 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("[circuit-breaker] %s: %s → %s", name, from, to)
		},
	})
}

var TripBreaker = newBreaker("trip-service")
var DriverBreaker = newBreaker("driver-service")
