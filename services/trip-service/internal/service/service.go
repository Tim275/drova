package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"drova/services/trip-service/internal/domain"
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
	}
	return s.repo.CreateTrip(ctx, trip)
}

func (s *service) GetRoute(ctx context.Context, pickup, destination string) (*types.MapboxRouteResponse, error) {
	pickupCoord, err := geocode(pickup)
	if err != nil {
		return nil, fmt.Errorf("geocode pickup: %w", err)
	}

	destCoord, err := geocode(destination)
	if err != nil {
		return nil, fmt.Errorf("geocode destination: %w", err)
	}

	routeURL := fmt.Sprintf(
		"https://api.mapbox.com/directions/v5/mapbox/driving/%f,%f;%f,%f?overview=full&geometries=geojson&access_token=%s",
		pickupCoord[0], pickupCoord[1],
		destCoord[0], destCoord[1],
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
		return nil, fmt.Errorf("read directions response: %w", err)
	}

	var route types.MapboxRouteResponse
	if err := json.Unmarshal(body, &route); err != nil {
		return nil, fmt.Errorf("parse directions response: %w", err)
	}

	return &route, nil
}

func geocode(address string) ([]float64, error) {
	ctx := context.Background()
	encoded := url.PathEscape(address)
	geocodeURL := fmt.Sprintf(
		"https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json?country=de&limit=1&access_token=%s",
		encoded,
		mapboxToken,
	)

	req, _ := http.NewRequestWithContext(ctx, "GET", geocodeURL, nil)
	req.Header.Set("Referer", "http://localhost:8083")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read geocoding response: %w", err)
	}

	var result types.MapboxGeocodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse geocoding response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, fmt.Errorf("no results for address: %s", address)
	}

	return result.Features[0].Center, nil // [lng, lat]
}
