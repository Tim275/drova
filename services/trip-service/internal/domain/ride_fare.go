package domain

import (
	"time"

	tripTypes "drova/services/trip-service/pkg/types"
	pb "drova/shared/proto/trip"
)

const FareExpiryDuration = 5 * time.Minute

type RideFareModel struct {
	ID                string
	UserID            string
	RiderName         string
	RiderAvatar       string
	PackageSlug       string
	TotalPriceInCents int64
	Route             *tripTypes.MapboxRouteResponse
	ExpiresAt         time.Time
	PickupAddress     string
	DropoffAddress    string
	PickupLat         float64
	PickupLng         float64
	DropoffLat        float64
	DropoffLng        float64
	DistanceMeters    int32
	DurationSeconds   int32
}

func (r *RideFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                r.ID,
		UserID:            r.UserID,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceInCents,
		PickupAddress:     r.PickupAddress,
		DropoffAddress:    r.DropoffAddress,
		DistanceMeters:    r.DistanceMeters,
		DurationSeconds:   r.DurationSeconds,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFare {
	var protoFares []*pb.RideFare
	for _, f := range fares {
		protoFares = append(protoFares, f.ToProto())
	}
	return protoFares
}
