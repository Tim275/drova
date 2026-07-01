package main

import (
	"drova/shared/contracts"
	pb "drova/shared/proto/trip"
)

type previewTripRequest struct {
	UserID         string               `json:"userID"`
	Pickup         contracts.Coordinate `json:"pickup"`
	Destination    contracts.Coordinate `json:"destination"`
	PickupAddress  string               `json:"pickupAddress"`
	DropoffAddress string               `json:"dropoffAddress"`
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
