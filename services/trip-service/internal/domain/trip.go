package domain

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"drova/shared/types"
)

type TripModel struct {
	ID     primitive.ObjectID
	UserID string
	Status string
	Fare   *RideFareModel
}

// TripRepository — data persistence contract (implemented by repository layer)
type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
}

// TripService — business logic contract (implemented by service layer)
type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination string) (*types.MapboxRouteResponse, error)
}
