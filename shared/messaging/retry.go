package messaging

import (
	"context"
	"log"
	"time"
)

// retryConfig + withBackoff were shared/retry. The Kafka client was the only
// consumer, so they now live inside the messaging package (unexported).
type retryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
}

func withBackoff(ctx context.Context, cfg retryConfig, operation func() error) error {
	var err error
	wait := cfg.InitialWait

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("retry attempt %d/%d after %v", attempt, cfg.MaxRetries, wait)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		}

		if err = operation(); err == nil {
			return nil
		}

		log.Printf("operation failed (attempt %d): %v", attempt+1, err)
	}

	return err
}
