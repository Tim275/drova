package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"drova/shared/messaging"
)

type pendingInfo struct {
	event    messaging.TripCreatedEvent
	driverID string
}

type TripConsumer struct {
	kafka    *messaging.Kafka
	service  *Service
	pending  sync.Map // tripID → pendingInfo
	waiting  sync.Map // tripID → TripCreatedEvent (no drivers found yet)
}

func NewTripConsumer(kafka *messaging.Kafka, service *Service) *TripConsumer {
	return &TripConsumer{kafka: kafka, service: service}
}

func (c *TripConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicTripCreated, "driver-service", c.handleFindAndNotifyDrivers)
	go c.kafka.ConsumeMessages(ctx, messaging.TopicDriverNotInterested, "driver-service-not-interested", c.handleFindAndNotifyDrivers)
	go c.kafka.ConsumeMessages(ctx, messaging.TopicDriverTripResponse, "driver-service-response-tracker", c.handleDriverResponse)
	go c.kafka.ConsumeMessages(ctx, messaging.TopicTripCancelled, "driver-service-cancellation", c.handleTripCancelled)
	go c.kafka.ConsumeMessages(ctx, messaging.TopicTripCompleted, "driver-service-completed", c.handleTripCompleted)
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

	appLog.Infow("trip search", "tripID", event.TripID, "package", event.PackageSlug, "type", msg.Type)

	drivers := c.service.FindAvailableDrivers(ctx, event.PackageSlug, event.Pickup.Lat, event.Pickup.Lng, event.ExcludeDriverIDs)
	if len(drivers) == 0 {
		appLog.Infow("no drivers available, queuing trip", "package", event.PackageSlug, "rider", event.UserID, "trip", event.TripID)
		c.waiting.Store(event.TripID, event)
		return c.publishNoDriversFound(ctx, event)
	}

	picked := drivers[0] // nearest driver first (sorted by haversine in FindAvailableDrivers)
	appLog.Infow("driver picked", "driver", picked.Id, "available", len(drivers), "package", event.PackageSlug, "distanceKm", "nearest")

	if err := c.notifyDriver(ctx, picked.Id, event); err != nil {
		return err
	}

	c.service.SetBusy(ctx, picked.Id, event.TripID)
	pi := pendingInfo{event: event, driverID: picked.Id}
	c.pending.Store(event.TripID, pi)

	go c.startResponseTimer(ctx, event.TripID, picked.Id, event)
	return nil
}

func (c *TripConsumer) startResponseTimer(ctx context.Context, tripID, driverID string, event messaging.TripCreatedEvent) {
	select {
	case <-time.After(15 * time.Second):
	case <-ctx.Done():
		return
	}
	val, loaded := c.pending.LoadAndDelete(tripID)
	if !loaded {
		return // driver already responded
	}
	pi := val.(pendingInfo)
	if pi.driverID != driverID {
		c.pending.Store(tripID, pi)
		return
	}
	appLog.Infow("driver timed out", "driver", driverID, "trip", tripID)
	c.service.ClearBusy(ctx, driverID)
	event.ExcludeDriverIDs = append(event.ExcludeDriverIDs, driverID)
	_ = c.publishRetry(ctx, event)
}

func (c *TripConsumer) handleDriverResponse(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	var data messaging.DriverTripResponseData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}
	c.pending.Delete(data.TripID) // stop the 15s response timer
	if msg.Type == "driver.cmd.trip_decline" {
		c.service.ClearBusy(ctx, data.Driver.ID)
	}
	return nil
}

func (c *TripConsumer) handleTripCancelled(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil
	}
	var data messaging.TripCancelledEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil
	}
	c.waiting.Delete(data.TripID)
	if val, loaded := c.pending.LoadAndDelete(data.TripID); loaded {
		pi := val.(pendingInfo)
		c.service.ClearBusy(ctx, pi.driverID)
		appLog.Infow("trip cancelled (pre-accept)", "trip", data.TripID, "driver", pi.driverID)
		return nil
	}
	if data.DriverID != "" {
		c.service.ClearBusy(ctx, data.DriverID)
		appLog.Infow("trip cancelled (post-accept)", "trip", data.TripID, "driver", data.DriverID)
	}
	return nil
}

func (c *TripConsumer) handleTripCompleted(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil
	}
	var data messaging.TripStatusEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil
	}
	if data.DriverID != "" {
		c.service.ClearBusy(ctx, data.DriverID)
		appLog.Infow("trip completed, driver freed", "trip", data.TripID, "driver", data.DriverID)
	}
	return nil
}

func (c *TripConsumer) TryMatchWaiting(ctx context.Context, packageSlug string, lat, lng float64) {
	c.waiting.Range(func(key, val any) bool {
		event := val.(messaging.TripCreatedEvent)
		if event.PackageSlug != packageSlug {
			return true
		}
		drivers := c.service.FindAvailableDrivers(ctx, event.PackageSlug, event.Pickup.Lat, event.Pickup.Lng, event.ExcludeDriverIDs)
		if len(drivers) == 0 {
			return true
		}
		c.waiting.Delete(key)
		picked := drivers[0] // nearest first
		appLog.Infow("matched waiting trip to new driver", "trip", event.TripID, "driver", picked.Id)
		if err := c.notifyDriver(ctx, picked.Id, event); err != nil {
			appLog.Warnw("notify driver failed, restoring waiting trip", "err", err)
			c.waiting.Store(key, event)
			return true
		}
		c.service.SetBusy(ctx, picked.Id, event.TripID)
		c.pending.Store(event.TripID, pendingInfo{event: event, driverID: picked.Id})
		go c.startResponseTimer(ctx, event.TripID, picked.Id, event)
		return true
	})
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

func (c *TripConsumer) publishRetry(ctx context.Context, event messaging.TripCreatedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	msg := messaging.KafkaMessage{
		Type:    messaging.TopicDriverNotInterested,
		OwnerID: event.UserID,
		Data:    data,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.kafka.PublishMessage(ctx, messaging.TopicDriverNotInterested, payload)
}
