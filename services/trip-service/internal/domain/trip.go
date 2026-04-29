package domain

import (
	"context"

	tripTypes "drova/services/trip-service/pkg/types"
	pbd "drova/shared/proto/driver"
	"drova/shared/types"
)

type TripModel struct {
	ID           string
	UserID       string
	RiderName    string
	RiderAvatar  string
	Status       string
	Fare         *RideFareModel
	DriverID     string
	DriverName   string
	DriverPlate  string
	DriverAvatar string
	Rating       int
	CreatedAt    int64
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, f *RideFareModel) error
	GetRideFareByID(ctx context.Context, id string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
	CancelTrip(ctx context.Context, tripID string) error
	ExpireSearch(ctx context.Context, tripID string) (bool, error)
	GetTripsByUser(ctx context.Context, userID string) ([]*TripModel, error)
	GetTripsByDriver(ctx context.Context, driverID string) ([]*TripModel, error)
	RateTrip(ctx context.Context, tripID string, rating int) error
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*tripTypes.MapboxRouteResponse, error)
	EstimatePackagesPriceWithRoute(route *tripTypes.MapboxRouteResponse) []*RideFareModel
	GenerateTripFares(ctx context.Context, fares []*RideFareModel, userID string, route *tripTypes.MapboxRouteResponse) ([]*RideFareModel, error)
	GetAndValidateFare(ctx context.Context, fareID, userID string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
	CancelTrip(ctx context.Context, tripID string) error
	ExpireSearch(ctx context.Context, tripID string) (bool, error)
	GetTripsByUser(ctx context.Context, userID string) ([]*TripModel, error)
	GetTripsByDriver(ctx context.Context, driverID string) ([]*TripModel, error)
	RateTrip(ctx context.Context, tripID string, rating int) error
}
