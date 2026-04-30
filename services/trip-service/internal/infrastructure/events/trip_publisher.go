package events

import (
	"context"
	"encoding/json"
	"fmt"

	"drova/services/trip-service/internal/domain"
	"drova/shared/messaging"

	"github.com/mmcloughlin/geohash"
	"go.uber.org/zap"
)

func (p *TripEventPublisher) PublishSearchTimeout(ctx context.Context, tripID, riderID string) error {
	noDriversData, err := json.Marshal(map[string]string{"trip_id": tripID})
	if err != nil {
		return fmt.Errorf("marshal no-drivers data: %w", err)
	}
	noDriversMsg := messaging.KafkaMessage{
		Type:    messaging.TopicTripNoDriversFound,
		OwnerID: riderID,
		Data:    noDriversData,
	}
	noDriversPayload, err := json.Marshal(noDriversMsg)
	if err != nil {
		return fmt.Errorf("marshal no-drivers msg: %w", err)
	}
	if err := p.kafka.PublishMessage(ctx, messaging.TopicTripNoDriversFound, noDriversPayload); err != nil {
		return fmt.Errorf("publish no-drivers-found: %w", err)
	}

	cancelData, err := json.Marshal(messaging.TripCancelledEvent{TripID: tripID, RiderID: riderID})
	if err != nil {
		return fmt.Errorf("marshal cancel event: %w", err)
	}
	cancelMsg := messaging.KafkaMessage{
		Type:    messaging.TopicTripCancelled,
		OwnerID: "",
		Data:    cancelData,
	}
	cancelPayload, err := json.Marshal(cancelMsg)
	if err != nil {
		return fmt.Errorf("marshal cancel msg: %w", err)
	}
	return p.kafka.PublishMessage(ctx, messaging.TopicTripCancelled, cancelPayload)
}

type TripEventPublisher struct {
	kafka *messaging.Kafka
	log   *zap.SugaredLogger
}

func NewTripEventPublisher(kafka *messaging.Kafka, log *zap.SugaredLogger) *TripEventPublisher {
	return &TripEventPublisher{kafka: kafka, log: log}
}

func (p *TripEventPublisher) PublishTripCreated(ctx context.Context, trip *domain.TripModel) error {
	rawCoords := trip.Fare.Route.Routes[0].Geometry.Coordinates
	pickup := messaging.Coordinate{Lat: rawCoords[0][1], Lng: rawCoords[0][0]}
	dest := messaging.Coordinate{Lat: rawCoords[len(rawCoords)-1][1], Lng: rawCoords[len(rawCoords)-1][0]}
	pickupGeohash := geohash.Encode(pickup.Lat, pickup.Lng)

	route := make([]messaging.Coordinate, len(rawCoords))
	for i, c := range rawCoords {
		route[i] = messaging.Coordinate{Lat: c[1], Lng: c[0]}
	}

	event := messaging.TripCreatedEvent{
		TripID:        trip.ID,
		UserID:        trip.UserID,
		RiderName:     trip.RiderName,
		RiderAvatar:   trip.RiderAvatar,
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

	p.log.Infow("trip created published", "trip", event.TripID, "user", event.UserID, "package", event.PackageSlug)
	return p.kafka.PublishMessage(ctx, messaging.TopicTripCreated, payload)
}
