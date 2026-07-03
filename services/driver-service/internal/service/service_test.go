package service

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "drova/shared/proto/driver"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Unit tests use miniredis (in-memory, pure Go). miniredis implements GEOADD but
// NOT GEOSEARCH, so the geo-matching paths (FindAvailableDrivers/GetNearbyDrivers)
// live in service_integration_test.go against a real Redis. Everything here covers
// the non-search orchestration: hash/heartbeat/geo writes, busy state, offline.

type stubStore struct {
	upserts []*pb.Driver
	err     error
}

func (s *stubStore) Upsert(_ context.Context, d *pb.Driver) error {
	if s.err != nil {
		return s.err
	}
	s.upserts = append(s.upserts, d)
	return nil
}

func setup(t *testing.T) (*Service, *miniredis.Miniredis, *redis.Client, *stubStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := &stubStore{}
	return NewService(store, rdb, zap.NewNop().Sugar()), mr, rdb, store
}

func exists(t *testing.T, rdb *redis.Client, key string) bool {
	t.Helper()
	n, err := rdb.Exists(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("exists %s: %v", key, err)
	}
	return n == 1
}

func inGeoSet(t *testing.T, rdb *redis.Client, pkg, driverID string) bool {
	t.Helper()
	err := rdb.ZScore(context.Background(), geoKey(pkg), driverID).Err()
	if err != nil && err != redis.Nil {
		t.Fatalf("zscore: %v", err)
	}
	return err == nil
}

// Düsseldorf Königsallee.
const (
	lat = 51.2277
	lng = 6.7735
)

// ── RegisterDriver ───────────────────────────────────────────────────────────

func TestRegisterDriver_StoresHashGeoAndHeartbeat(t *testing.T) {
	svc, _, rdb, store := setup(t)
	ctx := context.Background()

	d, err := svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Id != "d1" || d.Name != "Stefan" || d.PackageSlug != "economy" {
		t.Errorf("driver fields wrong: %+v", d)
	}
	if d.Location.Latitude != lat || d.Location.Longitude != lng {
		t.Errorf("location wrong: %+v", d.Location)
	}
	if len(store.upserts) != 1 {
		t.Errorf("want 1 store upsert, got %d", len(store.upserts))
	}
	if got := rdb.HGet(ctx, driverKey("d1"), "name").Val(); got != "Stefan" {
		t.Errorf("redis hash name = %q, want Stefan", got)
	}
	if !inGeoSet(t, rdb, "economy", "d1") {
		t.Error("driver not added to geo set")
	}
	if !exists(t, rdb, hbKey("d1")) {
		t.Error("heartbeat key not set")
	}
}

func TestRegisterDriver_EmptyNameDefaults(t *testing.T) {
	svc, _, _, _ := setup(t)
	d, _ := svc.RegisterDriver(context.Background(), "d2", "economy", "", lat, lng)
	if d.Name != "Driver" {
		t.Errorf("empty name should default to 'Driver', got %q", d.Name)
	}
}

func TestRegisterDriver_ZeroGPSUsesDemoLocation(t *testing.T) {
	svc, _, _, _ := setup(t)
	d, _ := svc.RegisterDriver(context.Background(), "d3", "economy", "X", 0, 0)
	if d.Location.Latitude == 0 && d.Location.Longitude == 0 {
		t.Error("zero GPS should fall back to a demo location, got 0,0")
	}
}

func TestRegisterDriver_StoreErrorIsTolerated(t *testing.T) {
	svc, _, _, store := setup(t)
	store.err = errors.New("db down")
	// Store failure is logged, not propagated — the driver still comes back from Redis state.
	d, err := svc.RegisterDriver(context.Background(), "d4", "economy", "X", lat, lng)
	if err != nil || d == nil {
		t.Errorf("store error must not fail registration, got err=%v", err)
	}
}

// ── Busy lifecycle ───────────────────────────────────────────────────────────

func TestBusyLifecycle(t *testing.T) {
	svc, _, _, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)

	if got := svc.GetBusyTripID(ctx, "d1"); got != "" {
		t.Errorf("fresh driver should not be busy, got %q", got)
	}
	svc.SetBusy(ctx, "d1", "trip-7")
	if got := svc.GetBusyTripID(ctx, "d1"); got != "trip-7" {
		t.Errorf("want trip-7, got %q", got)
	}
	svc.ClearBusy(ctx, "d1", "trip-7")
	if got := svc.GetBusyTripID(ctx, "d1"); got != "" {
		t.Errorf("want empty after clear, got %q", got)
	}
}

func TestGetBusyTripID_UnknownDriver(t *testing.T) {
	svc, _, _, _ := setup(t)
	if got := svc.GetBusyTripID(context.Background(), "ghost"); got != "" {
		t.Errorf("unknown driver should yield empty busy, got %q", got)
	}
}

// ── UpdateLocation ───────────────────────────────────────────────────────────

func TestUpdateLocation_SkipsZero(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)
	rdb.Del(ctx, hbKey("d1"))

	svc.UpdateLocation(ctx, "d1", 0, 0) // no-op

	if exists(t, rdb, hbKey("d1")) {
		t.Error("zero-coordinate update must not refresh heartbeat")
	}
}

func TestUpdateLocation_RefreshesHeartbeat(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)
	rdb.Del(ctx, hbKey("d1"))

	svc.UpdateLocation(ctx, "d1", lat+0.01, lng+0.01)

	if !exists(t, rdb, hbKey("d1")) {
		t.Error("valid update should refresh heartbeat")
	}
}

func TestUpdateLocation_UnknownDriverNoop(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	// No registered driver → HGet packageSlug errors → early return, no heartbeat created.
	svc.UpdateLocation(context.Background(), "ghost", lat, lng)
	if exists(t, rdb, hbKey("ghost")) {
		t.Error("update for unknown driver should be a no-op")
	}
}

// ── GoOffline ────────────────────────────────────────────────────────────────

func TestGoOffline_RemovesIdleDriver(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)

	svc.GoOffline(ctx, "d1")

	if exists(t, rdb, hbKey("d1")) {
		t.Error("heartbeat should be removed on offline")
	}
	if inGeoSet(t, rdb, "economy", "d1") {
		t.Error("driver should be removed from geo set on offline")
	}
	if exists(t, rdb, driverKey("d1")) {
		t.Error("idle driver hash should be removed on offline")
	}
}

func TestGoOffline_KeepsBusyDriverHash(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)
	svc.SetBusy(ctx, "d1", "trip-1")

	svc.GoOffline(ctx, "d1")

	if exists(t, rdb, hbKey("d1")) {
		t.Error("heartbeat should still be removed on offline")
	}
	if !exists(t, rdb, driverKey("d1")) {
		t.Error("busy driver hash must be kept on offline (trip still active)")
	}
}

func TestUnregisterDriver_IsSoftNoop(t *testing.T) {
	svc, _, rdb, _ := setup(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "S", lat, lng)

	svc.UnregisterDriver(ctx, "d1") // soft: must NOT evict (survives WS blips)

	if !exists(t, rdb, driverKey("d1")) {
		t.Error("UnregisterDriver must be soft — driver hash should survive")
	}
	_ = time.Second
}
