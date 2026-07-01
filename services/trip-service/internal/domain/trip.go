package domain

import (
	"context"
	"errors"
	"time"

	tripTypes "drova/services/trip-service/pkg/types"
	"drova/shared/contracts"
	pbd "drova/shared/proto/driver"
)

var ErrNotFound = errors.New("not found")

type TripModel struct {
	ID              string
	UserID          string
	RiderName       string
	RiderAvatar     string
	Status          string
	Fare            *RideFareModel
	DriverID        string
	DriverName      string
	DriverPlate     string
	DriverAvatar    string
	Rating          int
	RiderRating     int
	PickupAddress   string
	DropoffAddress  string
	DistanceMeters  int32
	DurationSeconds int32
	PackageSlug     string
	AmountCents     int64
	CreatedAt       int64
	CompletedAt     *time.Time
	CancelledAt     *time.Time
	CancelledBy     string
}

type OutboxMessage struct {
	Topic   string
	Payload []byte
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	CreateTripWithOutbox(ctx context.Context, trip *TripModel, topic string, payload []byte) (*TripModel, error)
	SaveRideFare(ctx context.Context, f *RideFareModel) error
	GetRideFareByID(ctx context.Context, id string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
	CancelTrip(ctx context.Context, tripID string) error
	ExpireSearch(ctx context.Context, tripID string) (bool, error)
	ExpireSearchWithOutbox(ctx context.Context, tripID string, msgs []OutboxMessage) (bool, error)
	ExpireStaleSearching(ctx context.Context, olderThan time.Duration) (int64, error)
	GetTripsByUser(ctx context.Context, userID string) ([]*TripModel, error)
	GetTripsByDriver(ctx context.Context, driverID string) ([]*TripModel, error)
	RateTrip(ctx context.Context, tripID string, rating int) error
}

type UserInfoProvider interface {
	GetRiderInfo(ctx context.Context, userID string) (name, avatar string)
}

// RouteCache caches Mapbox Directions API responses to avoid repeated external calls.
type RouteCache interface {
	Get(ctx context.Context, key string) (*tripTypes.MapboxRouteResponse, bool)
	Set(ctx context.Context, key string, route *tripTypes.MapboxRouteResponse)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *contracts.Coordinate) (*tripTypes.MapboxRouteResponse, error)
	EstimatePackagesPriceWithRoute(route *tripTypes.MapboxRouteResponse) []*RideFareModel
	GenerateTripFares(ctx context.Context, fares []*RideFareModel, userID string, route *tripTypes.MapboxRouteResponse) ([]*RideFareModel, error)
	GetAndValidateFare(ctx context.Context, fareID, userID string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
	CancelTrip(ctx context.Context, tripID string) error
	ExpireSearch(ctx context.Context, tripID string) (bool, error)
	ExpireSearchWithOutbox(ctx context.Context, tripID string, msgs []OutboxMessage) (bool, error)
	ExpireStaleSearching(ctx context.Context, olderThan time.Duration) (int64, error)
	GetTripsByUser(ctx context.Context, userID string) ([]*TripModel, error)
	GetTripsByDriver(ctx context.Context, driverID string) ([]*TripModel, error)
	RateTrip(ctx context.Context, tripID string, rating int) error
}
