package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"drova/shared/messaging"

	"go.uber.org/zap"
)

type Indexer struct {
	kafka *messaging.Kafka
	es    *ES
	log   *zap.SugaredLogger
}

func New(kafka *messaging.Kafka, es *ES, log *zap.SugaredLogger) *Indexer {
	return &Indexer{kafka: kafka, es: es, log: log}
}

func (i *Indexer) Start(ctx context.Context) {
	i.kafka.ConsumeMessages(ctx, messaging.TopicTripCreated, "search-indexer-created", i.handleCreated)
	i.kafka.ConsumeMessages(ctx, messaging.TopicTripCompleted, "search-indexer-completed", i.handleCompleted)
	i.kafka.ConsumeMessages(ctx, messaging.TopicTripCancelled, "search-indexer-cancelled", i.handleCancelled)
}

type geoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type tripDoc struct {
	TripID         string    `json:"trip_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"`
	PackageSlug    string    `json:"package_slug"`
	PriceCents     int64     `json:"price_cents"`
	DistanceMeters int32     `json:"distance_meters"`
	PickupAddress  string    `json:"pickup_address"`
	DropoffAddress string    `json:"dropoff_address"`
	Pickup         *geoPoint `json:"pickup,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func decode(payload []byte, out any) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	return json.Unmarshal(msg.Data, out)
}

func (i *Indexer) handleCreated(ctx context.Context, payload []byte) error {
	var ev messaging.TripCreatedEvent
	if err := decode(payload, &ev); err != nil {
		return err
	}
	doc := tripDoc{
		TripID:         ev.TripID,
		UserID:         ev.UserID,
		Status:         "searching",
		PackageSlug:    ev.PackageSlug,
		PriceCents:     ev.PriceCents,
		DistanceMeters: ev.DistanceMeters,
		PickupAddress:  ev.PickupAddress,
		DropoffAddress: ev.DropoffAddress,
		Pickup:         &geoPoint{Lat: ev.Pickup.Lat, Lon: ev.Pickup.Lng},
		CreatedAt:      time.Now().UTC(),
	}
	if err := i.es.IndexTrip(ctx, ev.TripID, doc); err != nil {
		return err
	}
	i.log.Infow("indexed trip", "trip", ev.TripID, "status", "searching")
	return nil
}

func (i *Indexer) handleCompleted(ctx context.Context, payload []byte) error {
	var ev messaging.TripStatusEvent
	if err := decode(payload, &ev); err != nil {
		return err
	}
	return i.es.UpdateTrip(ctx, ev.TripID, map[string]any{
		"trip_id":      ev.TripID,
		"status":       "completed",
		"driver_id":    ev.DriverID,
		"completed_at": time.Now().UTC(),
	})
}

func (i *Indexer) handleCancelled(ctx context.Context, payload []byte) error {
	var ev messaging.TripCancelledEvent
	if err := decode(payload, &ev); err != nil {
		return err
	}
	return i.es.UpdateTrip(ctx, ev.TripID, map[string]any{
		"trip_id": ev.TripID,
		"status":  "cancelled",
	})
}
