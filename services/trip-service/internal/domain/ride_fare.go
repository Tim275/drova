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
	TotalPriceInCents float64
	Route             *tripTypes.MapboxRouteResponse
	ExpiresAt         time.Time
}

func (r *RideFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                r.ID,
		UserID:            r.UserID,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceInCents,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFare {
	var protoFares []*pb.RideFare
	for _, f := range fares {
		protoFares = append(protoFares, f.ToProto())
	}
	return protoFares
}
