package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"drova/services/trip-service/internal/domain"
	tripTypes "drova/services/trip-service/pkg/types"
	"drova/shared/types"
)

// ── delegating repo methods ──────────────────────────────────────────────────

func TestGetTripByID(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	trip, _ := svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})

	got, err := svc.GetTripByID(context.Background(), trip.ID)
	if err != nil || got.ID != trip.ID {
		t.Fatalf("want trip %s, got %+v err=%v", trip.ID, got, err)
	}
	if _, err := svc.GetTripByID(context.Background(), "missing"); err == nil {
		t.Error("want error for unknown trip")
	}
}

func TestUpdateTrip(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	trip, _ := svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})

	if err := svc.UpdateTrip(context.Background(), trip.ID, "accepted", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.GetTripByID(context.Background(), trip.ID)
	if got.Status != "accepted" {
		t.Errorf("want status accepted, got %q", got.Status)
	}
}

func TestExpireSearchWithOutbox(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	trip, _ := svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})

	expired, err := svc.ExpireSearchWithOutbox(context.Background(), trip.ID, nil)
	if err != nil || !expired {
		t.Fatalf("want expired=true, got %v err=%v", expired, err)
	}
}

func TestExpireStaleSearching(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})
	svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u2"})

	n, err := svc.ExpireStaleSearching(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 expired searching trips, got %d", n)
	}
}

func TestGetTripsByUser(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})
	svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})
	svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u2"})

	got, err := svc.GetTripsByUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 trips for u1, got %d", len(got))
	}
}

func TestGetTripsByDriver(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)
	trip, _ := svc.CreateTrip(context.Background(), &domain.RideFareModel{UserID: "u1"})
	repo.trips[trip.ID].DriverID = "drv-7"

	got, err := svc.GetTripsByDriver(context.Background(), "drv-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 trip for drv-7, got %d", len(got))
	}
}

// ── GetRoute (cache + HTTP via overridable package httpClient) ────────────────

type stubCache struct {
	store map[string]*tripTypes.MapboxRouteResponse
	sets  int
}

func newStubCache() *stubCache {
	return &stubCache{store: make(map[string]*tripTypes.MapboxRouteResponse)}
}
func (c *stubCache) Get(_ context.Context, key string) (*tripTypes.MapboxRouteResponse, bool) {
	r, ok := c.store[key]
	return r, ok
}
func (c *stubCache) Set(_ context.Context, key string, route *tripTypes.MapboxRouteResponse) {
	c.store[key] = route
	c.sets++
}

type stubTransport struct {
	status int
	body   string
}

func (s stubTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

// withHTTPClient swaps the package httpClient for a stub and returns a restore fn.
func withHTTPClient(status int, body string) func() {
	prev := httpClient
	httpClient = &http.Client{Transport: stubTransport{status, body}}
	return func() { httpClient = prev }
}

func coord(latv, lngv float64) *types.Coordinate {
	return &types.Coordinate{Latitude: latv, Longitude: lngv}
}

func TestRouteCacheKey(t *testing.T) {
	k := routeCacheKey(coord(51.2277, 6.7735), coord(51.215, 6.79))
	if !strings.HasPrefix(k, "mapbox:route:") {
		t.Errorf("unexpected cache key: %s", k)
	}
}

func TestGetRoute_CacheHit(t *testing.T) {
	cache := newStubCache()
	pickup, dest := coord(51.2277, 6.7735), coord(51.215, 6.79)
	cached := route(1234, 99)
	cache.store[routeCacheKey(pickup, dest)] = cached

	svc := NewService(newFakeRepo(), &noopUsers{}, cache, nil)
	got, err := svc.GetRoute(context.Background(), pickup, dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != cached {
		t.Error("cache hit should return the cached route without an HTTP call")
	}
}

func TestGetRoute_HTTPSuccessCachesResult(t *testing.T) {
	defer withHTTPClient(200, `{"routes":[{"distance":5000,"duration":600}]}`)()
	cache := newStubCache()
	svc := NewService(newFakeRepo(), &noopUsers{}, cache, nil)

	got, err := svc.GetRoute(context.Background(), coord(51.2, 6.7), coord(51.3, 6.8))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Distance != 5000 {
		t.Errorf("unexpected route: %+v", got)
	}
	if cache.sets != 1 {
		t.Errorf("successful fetch should cache the result, sets=%d", cache.sets)
	}
}

func TestGetRoute_HTTPBadStatus(t *testing.T) {
	defer withHTTPClient(401, `{"message":"unauthorized"}`)()
	svc := NewService(newFakeRepo(), &noopUsers{}, nil, nil)
	if _, err := svc.GetRoute(context.Background(), coord(51.2, 6.7), coord(51.3, 6.8)); err == nil {
		t.Fatal("expected error on non-200 status (surfaces auth/quota errors)")
	}
}

func TestGetRoute_HTTPNoRoutes(t *testing.T) {
	defer withHTTPClient(200, `{"routes":[]}`)()
	svc := NewService(newFakeRepo(), &noopUsers{}, nil, nil)
	if _, err := svc.GetRoute(context.Background(), coord(51.2, 6.7), coord(51.3, 6.8)); err == nil {
		t.Fatal("expected 'no routes found' error")
	}
}
