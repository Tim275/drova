package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"drova/services/trip-service/internal/domain"
	"drova/shared/messaging"

	"github.com/mmcloughlin/geohash"
)

type TripEventPublisher struct {
	kafka *messaging.Kafka
}

func NewTripEventPublisher(kafka *messaging.Kafka) *TripEventPublisher {
	return &TripEventPublisher{kafka: kafka}
}

func (p *TripEventPublisher) PublishTripCreated(ctx context.Context, trip *domain.TripModel) error {
	rawCoords := trip.Fare.Route.Routes[0].Geometry.Coordinates
	// Mapbox coordinates are [longitude, latitude]
	pickup := messaging.Coordinate{Lat: rawCoords[0][1], Lng: rawCoords[0][0]}
	dest := messaging.Coordinate{Lat: rawCoords[len(rawCoords)-1][1], Lng: rawCoords[len(rawCoords)-1][0]}
	pickupGeohash := geohash.Encode(pickup.Lat, pickup.Lng)

	route := make([]messaging.Coordinate, len(rawCoords))
	for i, c := range rawCoords {
		route[i] = messaging.Coordinate{Lat: c[1], Lng: c[0]}
	}

	event := messaging.TripCreatedEvent{
		TripID:        trip.ID.Hex(),
		UserID:        trip.UserID,
		PackageSlug:   trip.Fare.PackageSlug,
		PickupGeohash: pickupGeohash,
		Pickup:        pickup,
		Destination:   dest,
		Route:         route,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := messaging.KafkaMessage{
		Type:    messaging.TopicTripCreated,
		OwnerID: trip.UserID,
		Data:    data,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	log.Printf("Publishing trip.event.created: tripID=%s userID=%s package=%s", event.TripID, event.UserID, event.PackageSlug)
	return p.kafka.PublishMessage(ctx, messaging.TopicTripCreated, payload)
}
