package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"drova/services/trip-service/internal/domain"
	tripTypes "drova/services/trip-service/pkg/types"
	"drova/shared/env"
	pbd "drova/shared/proto/driver"
	"drova/shared/types"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

var mapboxToken = env.GetString("MAPBOX_TOKEN", "")

var httpClient = &http.Client{Timeout: 10 * time.Second}

type service struct {
	repo       domain.TripRepository
	users      domain.UserInfoProvider
	cache      domain.RouteCache
	buildEvent func(*domain.TripModel) (string, []byte, error)
}

func NewService(repo domain.TripRepository, users domain.UserInfoProvider, cache domain.RouteCache, buildEvent func(*domain.TripModel) (string, []byte, error)) domain.TripService {
	return &service{repo: repo, users: users, cache: cache, buildEvent: buildEvent}
}

func (s *service) CreateTrip(ctx context.Context, fare *domain.RideFareModel) (*domain.TripModel, error) {
	trip := &domain.TripModel{
		ID:              uuid.New().String(),
		UserID:          fare.UserID,
		RiderName:       fare.RiderName,
		RiderAvatar:     fare.RiderAvatar,
		Status:          "searching",
		Fare:            fare,
		CreatedAt:       time.Now().Unix(),
		PickupAddress:   fare.PickupAddress,
		DropoffAddress:  fare.DropoffAddress,
		DistanceMeters:  fare.DistanceMeters,
		DurationSeconds: fare.DurationSeconds,
		PackageSlug:     fare.PackageSlug,
		AmountCents:     fare.TotalPriceInCents,
	}
	if s.buildEvent != nil {
		topic, payload, err := s.buildEvent(trip)
		if err != nil {
			return nil, err
		}
		return s.repo.CreateTripWithOutbox(ctx, trip, topic, payload)
	}
	return s.repo.CreateTrip(ctx, trip)
}

func routeCacheKey(pickup, dest *types.Coordinate) string {
	// 4 decimal places ≈ 11m precision — good enough to reuse route responses.
	return fmt.Sprintf("mapbox:route:%.4f:%.4f:%.4f:%.4f",
		pickup.Latitude, pickup.Longitude, dest.Latitude, dest.Longitude)
}

func (s *service) GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*tripTypes.MapboxRouteResponse, error) {
	cacheKey := routeCacheKey(pickup, destination)
	if s.cache != nil {
		if cached, ok := s.cache.Get(ctx, cacheKey); ok {
			return cached, nil
		}
	}

	routeURL := fmt.Sprintf(
		"https://api.mapbox.com/directions/v5/mapbox/driving/%f,%f;%f,%f?overview=full&geometries=geojson&access_token=%s",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
		mapboxToken,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", routeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create directions request: %w", err)
	}
	req.Header.Set("Referer", env.GetString("SERVICE_REFERER", "http://localhost:8083"))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("directions api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Surface auth/quota errors instead of masking them as "no routes found".
	// A bad MAPBOX_TOKEN returns 401/403 with no routes field, which would
	// otherwise be swallowed by the len(route.Routes)==0 check below.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("directions api status %d: %s", resp.StatusCode, string(body))
	}

	var route tripTypes.MapboxRouteResponse
	if err := json.Unmarshal(body, &route); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(route.Routes) == 0 {
		return nil, fmt.Errorf("no routes found for given coordinates")
	}

	if s.cache != nil {
		s.cache.Set(ctx, cacheKey, &route)
	}

	return &route, nil
}

func (s *service) EstimatePackagesPriceWithRoute(route *tripTypes.MapboxRouteResponse) []*domain.RideFareModel {
	baseFares := getBaseFares()
	estimatedFares := make([]*domain.RideFareModel, len(baseFares))
	for i, f := range baseFares {
		estimatedFares[i] = estimateFareRoute(f, route)
	}
	return estimatedFares
}

func (s *service) GenerateTripFares(ctx context.Context, rideFares []*domain.RideFareModel, userID string, route *tripTypes.MapboxRouteResponse) ([]*domain.RideFareModel, error) {
	riderName, riderAvatar := s.users.GetRiderInfo(ctx, userID)

	var distMeters int32
	var durSecs int32
	if len(route.Routes) > 0 {
		distMeters = int32(math.Round(route.Routes[0].Distance))
		durSecs = int32(math.Round(route.Routes[0].Duration))
	}

	fares := make([]*domain.RideFareModel, len(rideFares))
	for i, f := range rideFares {
		fares[i] = &domain.RideFareModel{
			ID:                uuid.New().String(),
			UserID:            userID,
			RiderName:         riderName,
			RiderAvatar:       riderAvatar,
			PackageSlug:       f.PackageSlug,
			TotalPriceInCents: f.TotalPriceInCents,
			Route:             route,
			ExpiresAt:         time.Now().Add(domain.FareExpiryDuration),
			PickupAddress:     rideFares[0].PickupAddress,
			DropoffAddress:    rideFares[0].DropoffAddress,
			PickupLat:         rideFares[0].PickupLat,
			PickupLng:         rideFares[0].PickupLng,
			DropoffLat:        rideFares[0].DropoffLat,
			DropoffLng:        rideFares[0].DropoffLng,
			DistanceMeters:    distMeters,
			DurationSeconds:   durSecs,
		}
	}

	// Write all 4 fares in parallel — they're independent inserts.
	g, gCtx := errgroup.WithContext(ctx)
	for _, fare := range fares {
		fare := fare
		g.Go(func() error {
			return s.repo.SaveRideFare(gCtx, fare)
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("save ride fare: %w", err)
	}
	return fares, nil
}

func (s *service) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	return s.repo.GetTripByID(ctx, id)
}

func (s *service) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	return s.repo.UpdateTrip(ctx, tripID, status, driver)
}

func (s *service) CancelTrip(ctx context.Context, tripID string) error {
	return s.repo.CancelTrip(ctx, tripID)
}

func (s *service) ExpireSearch(ctx context.Context, tripID string) (bool, error) {
	return s.repo.ExpireSearch(ctx, tripID)
}

func (s *service) ExpireSearchWithOutbox(ctx context.Context, tripID string, msgs []domain.OutboxMessage) (bool, error) {
	return s.repo.ExpireSearchWithOutbox(ctx, tripID, msgs)
}

func (s *service) ExpireStaleSearching(ctx context.Context, olderThan time.Duration) (int64, error) {
	return s.repo.ExpireStaleSearching(ctx, olderThan)
}

func (s *service) GetTripsByUser(ctx context.Context, userID string) ([]*domain.TripModel, error) {
	return s.repo.GetTripsByUser(ctx, userID)
}

func (s *service) GetTripsByDriver(ctx context.Context, driverID string) ([]*domain.TripModel, error) {
	return s.repo.GetTripsByDriver(ctx, driverID)
}

func (s *service) RateTrip(ctx context.Context, tripID string, rating int) error {
	return s.repo.RateTrip(ctx, tripID, rating)
}

func (s *service) GetAndValidateFare(ctx context.Context, fareID, userID string) (*domain.RideFareModel, error) {
	fare, err := s.repo.GetRideFareByID(ctx, fareID)
	if err != nil {
		return nil, fmt.Errorf("fare not found: %w", err)
	}
	if fare.UserID != userID {
		return nil, fmt.Errorf("fare does not belong to user")
	}
	if time.Now().After(fare.ExpiresAt) {
		return nil, fmt.Errorf("fare expired: please request a new quote")
	}
	return fare, nil
}

func estimateFareRoute(f *domain.RideFareModel, route *tripTypes.MapboxRouteResponse) *domain.RideFareModel {
	cfg := tripTypes.DefaultPricingConfig()
	distanceKm := route.Routes[0].Distance / 1000
	durationMin := route.Routes[0].Duration / 60
	total := float64(f.TotalPriceInCents) + distanceKm*cfg.PricePerUnitOfDistance + durationMin*cfg.PricingPerMinute
	return &domain.RideFareModel{
		PackageSlug:       f.PackageSlug,
		TotalPriceInCents: int64(math.Round(total)),
	}
}

func getBaseFares() []*domain.RideFareModel {
	return []*domain.RideFareModel{
		{PackageSlug: "economy", TotalPriceInCents: 500},
		{PackageSlug: "comfort", TotalPriceInCents: 800},
		{PackageSlug: "van", TotalPriceInCents: 1000},
		{PackageSlug: "business", TotalPriceInCents: 1500},
	}
}
