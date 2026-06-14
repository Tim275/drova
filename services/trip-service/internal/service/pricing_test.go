package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"drova/services/trip-service/internal/domain"
	tripTypes "drova/services/trip-service/pkg/types"
	pbd "drova/shared/proto/driver"
)

var errNotFound = errors.New("not found")

type noopUsers struct{}

func (n *noopUsers) GetRiderInfo(_ context.Context, _ string) (string, string) { return "", "" }

type fakeRepo struct {
	mu        sync.Mutex
	fares     map[string]*domain.RideFareModel
	trips     map[string]*domain.TripModel
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		fares: make(map[string]*domain.RideFareModel),
		trips: make(map[string]*domain.TripModel),
	}
}

func (f *fakeRepo) SaveRideFare(_ context.Context, fare *domain.RideFareModel) error {
	f.mu.Lock()
	f.fares[fare.ID] = fare
	f.mu.Unlock()
	return nil
}
func (f *fakeRepo) GetRideFareByID(_ context.Context, id string) (*domain.RideFareModel, error) {
	fare, ok := f.fares[id]
	if !ok {
		return nil, errNotFound
	}
	return fare, nil
}
func (f *fakeRepo) CreateTrip(_ context.Context, t *domain.TripModel) (*domain.TripModel, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.trips[t.ID] = t
	return t, nil
}
func (f *fakeRepo) CreateTripWithOutbox(ctx context.Context, t *domain.TripModel, _ string, _ []byte) (*domain.TripModel, error) {
	return f.CreateTrip(ctx, t)
}
func (f *fakeRepo) ExpireSearchWithOutbox(ctx context.Context, tripID string, _ []domain.OutboxMessage) (bool, error) {
	return f.ExpireSearch(ctx, tripID)
}
func (f *fakeRepo) GetTripByID(_ context.Context, id string) (*domain.TripModel, error) {
	t, ok := f.trips[id]
	if !ok {
		return nil, errNotFound
	}
	return t, nil
}
func (f *fakeRepo) UpdateTrip(_ context.Context, tripID, status string, _ *pbd.Driver) error {
	t, ok := f.trips[tripID]
	if !ok {
		return errNotFound
	}
	t.Status = status
	return nil
}
func (f *fakeRepo) CancelTrip(_ context.Context, tripID string) error {
	t, ok := f.trips[tripID]
	if !ok {
		return errNotFound
	}
	t.Status = "cancelled"
	return nil
}
func (f *fakeRepo) ExpireSearch(_ context.Context, tripID string) (bool, error) {
	t, ok := f.trips[tripID]
	if !ok {
		return false, nil
	}
	if t.Status == "searching" {
		t.Status = "expired"
		return true, nil
	}
	return false, nil
}
func (f *fakeRepo) ExpireStaleSearching(_ context.Context, _ time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, t := range f.trips {
		if t.Status == "searching" {
			t.Status = "expired"
			n++
		}
	}
	return n, nil
}
func (f *fakeRepo) GetTripsByUser(_ context.Context, userID string) ([]*domain.TripModel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.TripModel
	for _, t := range f.trips {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeRepo) GetTripsByDriver(_ context.Context, driverID string) ([]*domain.TripModel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.TripModel
	for _, t := range f.trips {
		if t.DriverID == driverID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeRepo) RateTrip(_ context.Context, tripID string, rating int) error {
	t, ok := f.trips[tripID]
	if !ok {
		return errNotFound
	}
	t.Rating = rating
	return nil
}

func route(distanceM, durationS float64) *tripTypes.MapboxRouteResponse {
	return &tripTypes.MapboxRouteResponse{
		Routes: []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		}{
			{Distance: distanceM, Duration: durationS},
		},
	}
}

func TestEstimateFareRoute(t *testing.T) {
	tests := []struct {
		name      string
		base      int64
		slug      string
		distanceM float64
		durationS float64
		wantCents int64
	}{
		{
			name:      "economy 10km 10min",
			slug:      "economy",
			base:      500,
			distanceM: 10_000,
			durationS: 600,
			wantCents: 500 + 10*150 + 10*30, // 2300
		},
		{
			name:      "business 5km 20min",
			slug:      "business",
			base:      1500,
			distanceM: 5_000,
			durationS: 1_200,
			wantCents: 1500 + 5*150 + 20*30, // 2850
		},
		{
			name:      "minimum fare zero distance",
			slug:      "economy",
			base:      500,
			distanceM: 0,
			durationS: 0,
			wantCents: 500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fare := &domain.RideFareModel{PackageSlug: tc.slug, TotalPriceInCents: tc.base}
			got := estimateFareRoute(fare, route(tc.distanceM, tc.durationS))

			if got.TotalPriceInCents != tc.wantCents {
				t.Errorf("want %d cents, got %d cents", tc.wantCents, got.TotalPriceInCents)
			}
			if got.PackageSlug != tc.slug {
				t.Errorf("want slug %q, got %q", tc.slug, got.PackageSlug)
			}
		})
	}
}

func TestEstimatePackagesPriceWithRoute_AllPackages(t *testing.T) {
	svc := NewService(newFakeRepo(), &noopUsers{}, nil, nil)
	fares := svc.EstimatePackagesPriceWithRoute(route(10_000, 600))

	wantPackages := []string{"economy", "comfort", "van", "business"}
	if len(fares) != len(wantPackages) {
		t.Fatalf("want %d packages, got %d", len(wantPackages), len(fares))
	}
	for i, want := range wantPackages {
		if fares[i].PackageSlug != want {
			t.Errorf("position %d: want %q, got %q", i, want, fares[i].PackageSlug)
		}
		if fares[i].TotalPriceInCents <= 0 {
			t.Errorf("package %q: price must be positive, got %d", want, fares[i].TotalPriceInCents)
		}
	}
}

func TestEstimatePackagesPriceWithRoute_PriceOrdering(t *testing.T) {
	svc := NewService(newFakeRepo(), &noopUsers{}, nil, nil)
	fares := svc.EstimatePackagesPriceWithRoute(route(10_000, 600))

	for i := 1; i < len(fares); i++ {
		if fares[i].TotalPriceInCents <= fares[i-1].TotalPriceInCents {
			t.Errorf("expected ascending prices: %s (%d) should cost more than %s (%d)",
				fares[i].PackageSlug, fares[i].TotalPriceInCents,
				fares[i-1].PackageSlug, fares[i-1].TotalPriceInCents,
			)
		}
	}
}

func TestGetAndValidateFare(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &noopUsers{}, nil, nil)

	validFare := &domain.RideFareModel{
		ID:        "fare-1",
		UserID:    "user-42",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	repo.fares["fare-1"] = validFare

	expiredFare := &domain.RideFareModel{
		ID:        "fare-expired",
		UserID:    "user-42",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	repo.fares["fare-expired"] = expiredFare

	tests := []struct {
		name    string
		fareID  string
		userID  string
		wantErr bool
	}{
		{"valid fare", "fare-1", "user-42", false},
		{"fare not found", "fare-missing", "user-42", true},
		{"wrong user", "fare-1", "user-99", true},
		{"expired fare", "fare-expired", "user-42", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetAndValidateFare(context.Background(), tc.fareID, tc.userID)
			if tc.wantErr && err == nil {
				t.Error("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("want no error, got %v", err)
			}
		})
	}
}
