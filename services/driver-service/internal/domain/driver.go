package domain

import (
	"context"

	pb "drova/shared/proto/driver"
)

type DriverStore interface {
	Upsert(ctx context.Context, d *pb.Driver) error
}

// DriverServicer is the subset of Service used by TripConsumer — allows unit testing without Redis.
type DriverServicer interface {
	FindAvailableDrivers(ctx context.Context, packageSlug string, lat, lng float64, exclude []string) []*pb.Driver
	SetBusy(ctx context.Context, driverID, tripID string)
	ClearBusy(ctx context.Context, driverID string)
}
