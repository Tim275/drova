package main

import (
	pb "drova/shared/proto/trip"
	"drova/shared/types"
)

type previewTripRequest struct {
	UserID         string           `json:"userID"`
	Pickup         types.Coordinate `json:"pickup"`
	Destination    types.Coordinate `json:"destination"`
	PickupAddress  string           `json:"pickupAddress"`
	DropoffAddress string           `json:"dropoffAddress"`
}

func (p *previewTripRequest) toProto() *pb.PreviewTripRequest {
	return &pb.PreviewTripRequest{
		UserID: p.UserID,
		StartLocation: &pb.Coordinate{
			Latitude:  p.Pickup.Latitude,
			Longitude: p.Pickup.Longitude,
		},
		EndLocation: &pb.Coordinate{
			Latitude:  p.Destination.Latitude,
			Longitude: p.Destination.Longitude,
		},
		PickupAddress:  p.PickupAddress,
		DropoffAddress: p.DropoffAddress,
	}
}

type startTripRequest struct {
	RideFareID string `json:"rideFareID"`
	UserID     string `json:"userID"`
}

func (c *startTripRequest) toProto() *pb.CreateTripRequest {
	return &pb.CreateTripRequest{
		RideFareID: c.RideFareID,
		UserID:     c.UserID,
	}
}
