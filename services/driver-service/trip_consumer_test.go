package main

import (
	"context"
	"os"
	"testing"
	"time"

	"drova/shared/messaging"
	pb "drova/shared/proto/driver"

	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	// appLog is a package-level var initialised in main(); set a no-op logger for tests.
	appLog = zap.NewNop().Sugar()
	os.Exit(m.Run())
}

// --- mock ---

type mockDriverSvc struct {
	clearBusyCalled bool
	clearBusyID     string
}

func (m *mockDriverSvc) FindAvailableDrivers(_ context.Context, _ string, _, _ float64, _ []string) []*pb.Driver {
	return nil
}
func (m *mockDriverSvc) SetBusy(_ context.Context, _, _ string) {}
func (m *mockDriverSvc) ClearBusy(_ context.Context, driverID string) {
	m.clearBusyCalled = true
	m.clearBusyID = driverID
}

// --- helpers ---

func newTestConsumer(svc driverServicer) *TripConsumer {
	return &TripConsumer{service: svc}
}

func testEvent(tripID string) messaging.TripCreatedEvent {
	return messaging.TripCreatedEvent{TripID: tripID, UserID: "u1", PackageSlug: "economy"}
}

// --- tests ---

// Context cancelled before the timer fires → function returns immediately.
// Pending entry must NOT be deleted (cancel path leaves cleanup to the caller).
func TestStartResponseTimer_CancelPath(t *testing.T) {
	c := newTestConsumer(&mockDriverSvc{})
	event := testEvent("trip-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	pi := pendingInfo{event: event, driverID: "drv-1", cancel: cancel}
	c.pending.Store(event.TripID, pi)

	done := make(chan struct{})
	go func() {
		c.startResponseTimer(ctx, event.TripID, "drv-1", event)
		close(done)
	}()

	cancel() // fire immediately

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("startResponseTimer did not exit after context cancel")
	}

	if _, ok := c.pending.Load(event.TripID); !ok {
		t.Error("cancel path must not delete the pending entry — caller owns cleanup")
	}
}

// Timer fires (overridden to 30ms) → ClearBusy must be called for the driver.
// kafka.PublishMessage will panic on nil receiver; we recover and still verify ClearBusy.
func TestStartResponseTimer_Timeout_ClearsBusy(t *testing.T) {
	responseTimeout = 30 * time.Millisecond
	defer func() { responseTimeout = 15 * time.Second }()

	svc := &mockDriverSvc{}
	c := newTestConsumer(svc)
	event := testEvent("trip-timeout")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pi := pendingInfo{event: event, driverID: "drv-2", cancel: cancel}
	c.pending.Store(event.TripID, pi)

	done := make(chan struct{})
	go func() {
		defer func() { recover(); close(done) }() //nolint:errcheck // nil kafka panics; we only care about ClearBusy
		c.startResponseTimer(ctx, event.TripID, "drv-2", event)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timer did not fire within 500ms")
	}

	if !svc.clearBusyCalled {
		t.Error("ClearBusy must be called when driver times out")
	}
	if svc.clearBusyID != "drv-2" {
		t.Errorf("ClearBusy called with wrong driver: %s", svc.clearBusyID)
	}
}

// ExtendTimerForDriver returns false and does not modify state when the trip has no pending entry.
func TestExtendTimerForDriver_NotFound(t *testing.T) {
	c := newTestConsumer(&mockDriverSvc{})

	if c.ExtendTimerForDriver(context.Background(), "nonexistent-trip", "drv-1") {
		t.Error("must return false when no pending entry exists")
	}
}

// ExtendTimerForDriver returns false and restores the original entry when the
// driver ID does not match — another driver owns this pending slot.
func TestExtendTimerForDriver_WrongDriver(t *testing.T) {
	c := newTestConsumer(&mockDriverSvc{})
	event := testEvent("trip-wrong-drv")

	cancelCalled := false
	pi := pendingInfo{
		event:    event,
		driverID: "drv-correct",
		cancel:   func() { cancelCalled = true },
	}
	c.pending.Store(event.TripID, pi)

	result := c.ExtendTimerForDriver(context.Background(), event.TripID, "drv-WRONG")

	if result {
		t.Error("must return false for wrong driver ID")
	}
	if cancelCalled {
		t.Error("must not cancel the timer of a different driver")
	}

	val, ok := c.pending.Load(event.TripID)
	if !ok {
		t.Fatal("pending entry must be restored when driver ID does not match")
	}
	restored := val.(pendingInfo)
	if restored.driverID != "drv-correct" {
		t.Errorf("restored entry has wrong driverID: %q", restored.driverID)
	}
}
