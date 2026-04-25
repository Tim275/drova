package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"

	"drova/shared/messaging"
)

type TripConsumer struct {
	kafka   *messaging.Kafka
	service *Service
}

func NewTripConsumer(kafka *messaging.Kafka, service *Service) *TripConsumer {
	return &TripConsumer{kafka: kafka, service: service}
}

func (c *TripConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicTripCreated, "driver-service", c.handleFindAndNotifyDrivers)
	go c.kafka.ConsumeMessages(ctx, messaging.TopicDriverNotInterested, "driver-service-not-interested", c.handleFindAndNotifyDrivers)
}

func (c *TripConsumer) handleFindAndNotifyDrivers(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("unmarshal kafka message: %w", err)
	}

	var event messaging.TripCreatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return fmt.Errorf("unmarshal trip event: %w", err)
	}

	log.Printf("Trip search: tripID=%s package=%s type=%s", event.TripID, event.PackageSlug, msg.Type)

	drivers := c.service.FindAvailableDrivers(event.PackageSlug)
	if len(drivers) == 0 {
		log.Printf("No available drivers for package %s — notifying rider %s", event.PackageSlug, event.UserID)
		return c.publishNoDriversFound(ctx, event)
	}

	picked := drivers[rand.IntN(len(drivers))]
	log.Printf("Picked driver %s out of %d available for package %s", picked.Id, len(drivers), event.PackageSlug)

	return c.notifyDriver(ctx, picked.Id, event)
}

func (c *TripConsumer) publishNoDriversFound(ctx context.Context, event messaging.TripCreatedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	kmsg := messaging.KafkaMessage{
		Type:    messaging.TopicTripNoDriversFound,
		OwnerID: event.UserID,
		Data:    data,
	}
	payload, err := json.Marshal(kmsg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return c.kafka.PublishMessage(ctx, messaging.TopicTripNoDriversFound, payload)
}

func (c *TripConsumer) notifyDriver(ctx context.Context, driverID string, event messaging.TripCreatedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := messaging.KafkaMessage{
		Type:    messaging.TopicDriverTripRequest,
		OwnerID: driverID,
		Data:    data,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.kafka.PublishMessage(ctx, messaging.TopicDriverTripRequest, payload)
}
