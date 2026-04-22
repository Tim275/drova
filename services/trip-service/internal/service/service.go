package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"drova/services/trip-service/internal/domain"
	tripTypes "drova/services/trip-service/pkg/types"
	"drova/shared/env"
	"drova/shared/types"
)

var mapboxToken = env.GetString("MAPBOX_TOKEN", "")

type service struct {
	repo domain.TripRepository
}

func NewService(repo domain.TripRepository) domain.TripService {
	return &service{repo: repo}
}

func (s *service) CreateTrip(ctx context.Context, fare *domain.RideFareModel) (*domain.TripModel, error) {
	trip := &domain.TripModel{
		ID:     primitive.NewObjectID(),
		UserID: fare.UserID,
		Status: "pending",
		Fare:   fare,
		Driver: nil,
	}
	return s.repo.CreateTrip(ctx, trip)
}

func (s *service) GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*tripTypes.MapboxRouteResponse, error) {
	routeURL := fmt.Sprintf(
		"https://api.mapbox.com/directions/v5/mapbox/driving/%f,%f;%f,%f?overview=full&geometries=geojson&access_token=%s",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
		mapboxToken,
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", routeURL, nil)
	req.Header.Set("Referer", "http://localhost:8083")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("directions api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var route tripTypes.MapboxRouteResponse
	if err := json.Unmarshal(body, &route); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(route.Routes) == 0 {
		log.Printf("Mapbox returned no routes. Body: %s", string(body))
		return nil, fmt.Errorf("no routes found for given coordinates")
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

func (s *service) GenerateTripFares(ctx context.Context, rideFares []*domain.RideFareModel, userID string) ([]*domain.RideFareModel, error) {
	fares := make([]*domain.RideFareModel, len(rideFares))
	for i, f := range rideFares {
		fare := &domain.RideFareModel{
			ID:                primitive.NewObjectID(),
			UserID:            userID,
			PackageSlug:       f.PackageSlug,
			TotalPriceInCents: f.TotalPriceInCents,
		}
		if err := s.repo.SaveRideFare(ctx, fare); err != nil {
			return nil, fmt.Errorf("failed to save trip fare: %w", err)
		}
		fares[i] = fare
	}
	return fares, nil
}

func (s *service) GetAndValidateFare(ctx context.Context, fareID, userID string) (*domain.RideFareModel, error) {
	fare, err := s.repo.GetRideFareByID(ctx, fareID)
	if err != nil {
		return nil, fmt.Errorf("fare not found: %w", err)
	}
	if fare.UserID != userID {
		return nil, fmt.Errorf("fare does not belong to user")
	}
	return fare, nil
}

func estimateFareRoute(f *domain.RideFareModel, route *tripTypes.MapboxRouteResponse) *domain.RideFareModel {
	cfg := tripTypes.DefaultPricingConfig()
	distanceKm := route.Routes[0].Distance / 1000 // meters → km
	durationMin := route.Routes[0].Duration / 60  // seconds → minutes
	total := f.TotalPriceInCents + distanceKm*cfg.PricePerUnitOfDistance + durationMin*cfg.PricingPerMinute
	return &domain.RideFareModel{
		PackageSlug:       f.PackageSlug,
		TotalPriceInCents: total,
	}
}

func getBaseFares() []*domain.RideFareModel {
	return []*domain.RideFareModel{
		{PackageSlug: "sedan", TotalPriceInCents: 500},
		{PackageSlug: "suv", TotalPriceInCents: 800},
		{PackageSlug: "van", TotalPriceInCents: 1000},
		{PackageSlug: "luxury", TotalPriceInCents: 1500},
	}
}
