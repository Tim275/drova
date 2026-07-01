package messaging

import (
	"context"
	"errors"
	"testing"
	"time"
)

var fastRetryCfg = retryConfig{MaxRetries: 3, InitialWait: time.Millisecond, MaxWait: 2 * time.Millisecond}

func TestWithBackoff_SuccessFirstTry(t *testing.T) {
	calls := 0
	err := withBackoff(context.Background(), fastRetryCfg, func() error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWithBackoff_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := withBackoff(context.Background(), fastRetryCfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWithBackoff_Exhausted(t *testing.T) {
	want := errors.New("permanent")
	calls := 0
	err := withBackoff(context.Background(), retryConfig{MaxRetries: 2, InitialWait: time.Millisecond, MaxWait: time.Millisecond}, func() error {
		calls++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if calls != 3 { // initial + 2 retries
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := withBackoff(ctx, retryConfig{MaxRetries: 3, InitialWait: time.Hour, MaxWait: time.Hour}, func() error {
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
