//go:build integration

package service

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// These exercise the geo-matching paths (GEOSEARCH) which miniredis does not
// implement — they need a real Redis. Set TEST_REDIS_ADDR (CI provides one);
// otherwise they skip. Uses DB 15 + FlushDB so it never touches real data.

func setupReal(t *testing.T) (*Service, *redis.Client, *stubStore) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set — skipping Redis integration test")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis ping %s: %v", addr, err)
	}
	rdb.FlushDB(context.Background())
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		_ = rdb.Close()
	})
	return NewService(&stubStore{}, rdb, zap.NewNop().Sugar()), rdb, &stubStore{}
}

func TestFindAvailableDrivers_FindsRegistered(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)

	got := svc.FindAvailableDrivers(ctx, "economy", lat, lng, nil)
	if len(got) != 1 || got[0].Id != "d1" {
		t.Fatalf("want [d1], got %+v", got)
	}
	if got[0].Name != "Stefan" {
		t.Errorf("want name Stefan, got %q", got[0].Name)
	}
}

func TestFindAvailableDrivers_ExcludesID(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)
	svc.RegisterDriver(ctx, "d2", "economy", "Other", lat, lng)

	got := svc.FindAvailableDrivers(ctx, "economy", lat, lng, []string{"d1"})
	for _, d := range got {
		if d.Id == "d1" {
			t.Error("excluded driver d1 must not appear")
		}
	}
	if len(got) != 1 || got[0].Id != "d2" {
		t.Errorf("want only [d2], got %+v", got)
	}
}

func TestFindAvailableDrivers_SkipsBusy(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)
	svc.SetBusy(ctx, "d1", "trip-9")

	got := svc.FindAvailableDrivers(ctx, "economy", lat, lng, nil)
	if len(got) != 0 {
		t.Errorf("busy driver should be skipped, got %+v", got)
	}
}

func TestFindAvailableDrivers_ReapsExpiredHeartbeat(t *testing.T) {
	svc, rdb, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)
	rdb.Del(ctx, hbKey("d1")) // simulate heartbeat expiry

	got := svc.FindAvailableDrivers(ctx, "economy", lat, lng, nil)
	if len(got) != 0 {
		t.Errorf("driver with no heartbeat should not be returned, got %+v", got)
	}
	if inGeoSet(t, rdb, "economy", "d1") {
		t.Error("ghost geo entry should be reaped")
	}
	if exists(t, rdb, driverKey("d1")) {
		t.Error("ghost hash should be reaped (driver was not busy)")
	}
}

func TestFindAvailableDrivers_NoneWhenEmpty(t *testing.T) {
	svc, _, _ := setupReal(t)
	got := svc.FindAvailableDrivers(context.Background(), "economy", lat, lng, nil)
	if got != nil {
		t.Errorf("expected nil for empty geo set, got %+v", got)
	}
}

func TestFindAvailableDrivers_RadiusExpansion(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	// ~55 km north of pickup — misses the 5/25 km rings, found at 100 km.
	svc.RegisterDriver(ctx, "far", "economy", "Far", lat+0.5, lng)

	got := svc.FindAvailableDrivers(ctx, "economy", lat, lng, nil)
	if len(got) != 1 || got[0].Id != "far" {
		t.Fatalf("radius expansion should find distant driver, got %+v", got)
	}
}

func TestGetNearbyDrivers_DefaultRadius(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "Stefan", lat, lng)

	got := svc.GetNearbyDrivers(ctx, "economy", lat, lng, 0) // 0 → defaults to 10 km
	if len(got) != 1 || got[0].Id != "d1" {
		t.Fatalf("want [d1], got %+v", got)
	}
}

func TestGetNearbyDrivers_AllPackagesAndDedup(t *testing.T) {
	svc, _, _ := setupReal(t)
	ctx := context.Background()
	svc.RegisterDriver(ctx, "d1", "economy", "A", lat, lng)
	svc.RegisterDriver(ctx, "d2", "comfort", "B", lat+0.0001, lng+0.0001)

	got := svc.GetNearbyDrivers(ctx, "", lat, lng, 10) // "" → all base packages
	if len(got) != 2 {
		t.Errorf("want 2 drivers across packages, got %d (%+v)", len(got), got)
	}
}
