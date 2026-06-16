package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"drova/services/driver-service/internal/domain"
	"drova/services/driver-service/internal/sim"
	"drova/shared/env"
	"drova/shared/messaging"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const waitingTTL = 10 * time.Minute

// responseTimeout is the window a driver has to accept a trip request.
// Overridable in tests to avoid real 15s waits.
var responseTimeout = 15 * time.Second

type pendingInfo struct {
	event    messaging.TripCreatedEvent
	driverID string
	cancel   context.CancelFunc // cancels the 15s timer goroutine
}

type TripConsumer struct {
	kafka   *messaging.Kafka
	service domain.DriverServicer
	rdb     *redis.Client
	log     *zap.SugaredLogger
	sim     *sim.Simulator
	// autoDrive: when true the server-side sim auto-drives the trip on accept.
	// Default false = manual driver phases (arrived/start/end) like Uber/Bolt.
	autoDrive bool
	pending   sync.Map // tripID → pendingInfo (short-lived, in-memory is fine)
}

func NewTripConsumer(kafka *messaging.Kafka, service domain.DriverServicer, rdb *redis.Client, log *zap.SugaredLogger, simulator *sim.Simulator) *TripConsumer {
	return &TripConsumer{
		kafka: kafka, service: service, rdb: rdb, log: log, sim: simulator,
		autoDrive: env.GetString("DRIVER_SIM_AUTODRIVE", "") == "true",
	}
}

func geoKey(packageSlug string) string { return "drova:drivers:geo:" + packageSlug }
func driverKey(driverID string) string { return "drova:driver:" + driverID }

func (c *TripConsumer) waitingStore(ctx context.Context, event messaging.TripCreatedEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		c.log.Warnw("waiting store marshal", "trip", event.TripID, "err", err)
		return
	}
	pipe := c.rdb.Pipeline()
	pipe.Set(ctx, "drv:waiting:"+event.TripID, data, waitingTTL)
	pipe.SAdd(ctx, "drv:waiting:pkg:"+event.PackageSlug, event.TripID)
	if _, err := pipe.Exec(ctx); err != nil {
		c.log.Warnw("waiting store redis", "trip", event.TripID, "err", err)
	}
}

func (c *TripConsumer) waitingDelete(ctx context.Context, tripID string) {
	raw, err := c.rdb.Get(ctx, "drv:waiting:"+tripID).Result()
	if errors.Is(err, redis.Nil) {
		return
	}
	var event messaging.TripCreatedEvent
	if err := json.Unmarshal([]byte(raw), &event); err == nil {
		pipe := c.rdb.Pipeline()
		pipe.Del(ctx, "drv:waiting:"+tripID)
		pipe.SRem(ctx, "drv:waiting:pkg:"+event.PackageSlug, tripID)
		pipe.Exec(ctx) //nolint:errcheck
	}
}

// waitingClaim atomically removes the trip from Redis and returns the event.
// Returns nil if the trip expired or was already claimed by another instance.
func (c *TripConsumer) waitingClaim(ctx context.Context, tripID string) *messaging.TripCreatedEvent {
	raw, err := c.rdb.GetDel(ctx, "drv:waiting:"+tripID).Result()
	if err != nil {
		return nil
	}
	var event messaging.TripCreatedEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return nil
	}
	return &event
}

func (c *TripConsumer) Start(ctx context.Context) {
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripCreated, "driver-service", c.handleFindAndNotifyDrivers)
	c.kafka.ConsumeMessages(ctx, messaging.TopicDriverNotInterested, "driver-service-not-interested", c.handleFindAndNotifyDrivers)
	c.kafka.ConsumeMessages(ctx, messaging.TopicDriverTripResponse, "driver-service-response-tracker", c.handleDriverResponse)
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripCancelled, "driver-service-cancellation", c.handleTripCancelled)
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripCompleted, "driver-service-completed", c.handleTripCompleted)
	c.kafka.StartRetryConsumers(ctx, "driver-service")
}

func (c *TripConsumer) spawnTimer(parentCtx context.Context, event messaging.TripCreatedEvent, driverID string) {
	timerCtx, cancel := context.WithCancel(parentCtx)
	pi := pendingInfo{event: event, driverID: driverID, cancel: cancel}
	c.pending.Store(event.TripID, pi)
	go c.startResponseTimer(timerCtx, event.TripID, driverID, event) //nolint:gosec
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

	c.log.Infow("trip search", "tripID", event.TripID, "package", event.PackageSlug, "type", msg.Type)

	drivers := c.service.FindAvailableDrivers(ctx, event.PackageSlug, event.Pickup.Lat, event.Pickup.Lng, event.ExcludeDriverIDs)
	if len(drivers) == 0 {
		c.log.Infow("no drivers available, queuing trip", "package", event.PackageSlug, "rider", event.UserID, "trip", event.TripID)
		c.waitingStore(ctx, event)
		return c.publishNoDriversFound(ctx, event)
	}

	picked := drivers[0]
	c.log.Infow("driver picked", "driver", picked.Id, "available", len(drivers), "package", event.PackageSlug)

	if err := c.notifyDriver(ctx, picked.Id, event); err != nil {
		return err
	}

	c.service.SetBusy(ctx, picked.Id, event.TripID)
	c.spawnTimer(ctx, event, picked.Id)
	return nil
}

// startResponseTimer waits 15s for driver to respond. ctx.Done() means the timer was
// explicitly cancelled (driver responded or reconnect extension). Only the 15s path clears busy.
func (c *TripConsumer) startResponseTimer(ctx context.Context, tripID, driverID string, event messaging.TripCreatedEvent) {
	select {
	case <-time.After(responseTimeout):
	case <-ctx.Done():
		return // cancelled by handleDriverResponse, handleTripCancelled, or ExtendTimerForDriver
	}
	val, loaded := c.pending.LoadAndDelete(tripID)
	if !loaded {
		return // already handled by response/cancel path
	}
	pi, ok := val.(pendingInfo)
	if !ok {
		return
	}
	if pi.driverID != driverID {
		c.pending.Store(tripID, pi) // restore — different driver owns this entry
		return
	}
	c.log.Infow("driver timed out", "driver", driverID, "trip", tripID)
	// Use background context: timerCtx (ctx) must not be cancelled before cleanup finishes.
	// pi.cancel() would cancel ctx, so we call it last.
	bgCtx := context.Background()
	c.service.ClearBusy(bgCtx, driverID)
	event.ExcludeDriverIDs = append(event.ExcludeDriverIDs, driverID)
	_ = c.publishRetry(bgCtx, event)
	pi.cancel() // release context resources after cleanup
}

// ExtendTimerForDriver is called on driver reconnect. Returns true if the timer was found
// and extended, false if no pending entry exists (service restarted or trip already resolved).
func (c *TripConsumer) ExtendTimerForDriver(parentCtx context.Context, tripID, driverID string) bool {
	val, loaded := c.pending.LoadAndDelete(tripID) // atomically take
	if !loaded {
		return false // timer fired, driver responded, or service restarted
	}
	pi, ok := val.(pendingInfo)
	if !ok {
		return false
	}
	if pi.driverID != driverID {
		c.pending.Store(tripID, pi) // belongs to a different driver, put back
		return false
	}
	pi.cancel() // stop old timer goroutine
	c.log.Infow("timer extended on driver reconnect", "driver", driverID, "trip", tripID)
	c.spawnTimer(parentCtx, pi.event, driverID)
	// Re-send the trip request after a short delay so the driver's Redis pub/sub
	// subscription is active before the Kafka notification arrives.
	event := pi.event
	go func() { //nolint:gosec
		time.Sleep(400 * time.Millisecond)
		if err := c.notifyDriver(context.Background(), driverID, event); err != nil {
			c.log.Warnw("re-notify driver on reconnect", "driver", driverID, "trip", tripID, "err", err)
		}
	}()
	return true
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
	var accepted *messaging.TripCreatedEvent
	if val, loaded := c.pending.LoadAndDelete(data.TripID); loaded {
		if pi, ok := val.(pendingInfo); ok {
			pi.cancel() // stop the 15s timer goroutine
			ev := pi.event
			accepted = &ev
		}
	}
	if msg.Type == "driver.cmd.trip_decline" {
		c.service.ClearBusy(ctx, data.Driver.ID)
		return nil
	}
	if c.sim != nil && c.autoDrive && accepted != nil {
		startLat, startLng := c.driverPos(ctx, data.Driver.ID, accepted.PackageSlug)
		c.sim.Start(sim.TripPlan{
			TripID:      data.TripID,
			RiderID:     data.RiderID,
			DriverID:    data.Driver.ID,
			PackageSlug: accepted.PackageSlug,
			StartLat:    startLat,
			StartLng:    startLng,
			Pickup:      accepted.Pickup,
			Route:       accepted.Route,
		})
	}
	return nil
}

func (c *TripConsumer) driverPos(ctx context.Context, driverID, packageSlug string) (float64, float64) {
	if c.rdb == nil || packageSlug == "" {
		return 0, 0
	}
	pos, err := c.rdb.GeoPos(ctx, geoKey(packageSlug), driverID).Result()
	if err != nil || len(pos) == 0 || pos[0] == nil {
		return 0, 0
	}
	return pos[0].Latitude, pos[0].Longitude
}

func (c *TripConsumer) handleTripCancelled(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.log.Warnw("handleTripCancelled: unmarshal envelope", "err", err)
		return nil
	}
	var data messaging.TripCancelledEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.log.Warnw("handleTripCancelled: unmarshal data", "err", err)
		return nil
	}
	c.waitingDelete(ctx, data.TripID)
	if c.sim != nil {
		c.sim.Stop(data.TripID)
	}
	if val, loaded := c.pending.LoadAndDelete(data.TripID); loaded {
		pi, ok := val.(pendingInfo)
		if !ok {
			return nil
		}
		pi.cancel() // stop the 15s timer goroutine
		c.service.ClearBusy(ctx, pi.driverID)
		c.log.Infow("trip cancelled (pre-accept)", "trip", data.TripID, "driver", pi.driverID)
		return nil
	}
	if data.DriverID != "" {
		c.service.ClearBusy(ctx, data.DriverID)
		c.log.Infow("trip cancelled (post-accept)", "trip", data.TripID, "driver", data.DriverID)
		go c.tryMatchFreedDriver(context.Background(), data.DriverID) //nolint:gosec
	}
	return nil
}

func (c *TripConsumer) handleTripCompleted(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.log.Warnw("handleTripCompleted: unmarshal envelope", "err", err)
		return nil
	}
	var data messaging.TripStatusEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.log.Warnw("handleTripCompleted: unmarshal data", "err", err)
		return nil
	}
	if data.DriverID != "" {
		c.service.ClearBusy(ctx, data.DriverID)
		c.log.Infow("trip completed, driver freed", "trip", data.TripID, "driver", data.DriverID)
		go c.tryMatchFreedDriver(context.Background(), data.DriverID) //nolint:gosec
	}
	return nil
}

// tryMatchFreedDriver checks the waiting queue after a driver is freed (trip completed/cancelled).
func (c *TripConsumer) tryMatchFreedDriver(ctx context.Context, driverID string) {
	packageSlug, err := c.rdb.HGet(ctx, driverKey(driverID), "packageSlug").Result()
	if err != nil || packageSlug == "" {
		return
	}
	pos, err := c.rdb.GeoPos(ctx, geoKey(packageSlug), driverID).Result()
	if err != nil || len(pos) == 0 || pos[0] == nil {
		return
	}
	c.TryMatchWaiting(ctx, packageSlug, pos[0].Latitude, pos[0].Longitude)
}

func (c *TripConsumer) TryMatchWaiting(ctx context.Context, packageSlug string, lat, lng float64) {
	tripIDs, err := c.rdb.SMembers(ctx, "drv:waiting:pkg:"+packageSlug).Result()
	if err != nil || len(tripIDs) == 0 {
		return
	}
	for _, tripID := range tripIDs {
		event := c.waitingClaim(ctx, tripID)
		if event == nil {
			// TTL expired or claimed by another instance — clean up set
			c.rdb.SRem(ctx, "drv:waiting:pkg:"+packageSlug, tripID) //nolint:errcheck
			continue
		}
		c.rdb.SRem(ctx, "drv:waiting:pkg:"+packageSlug, tripID) //nolint:errcheck

		drivers := c.service.FindAvailableDrivers(ctx, event.PackageSlug, event.Pickup.Lat, event.Pickup.Lng, event.ExcludeDriverIDs)
		if len(drivers) == 0 {
			c.waitingStore(ctx, *event) // no driver yet — put it back
			continue
		}
		picked := drivers[0]
		c.log.Infow("matched waiting trip to new driver", "trip", event.TripID, "driver", picked.Id)
		if err := c.notifyDriver(ctx, picked.Id, *event); err != nil {
			c.log.Warnw("notify driver failed, restoring waiting trip", "err", err)
			c.waitingStore(ctx, *event)
			continue
		}
		c.service.SetBusy(ctx, picked.Id, event.TripID)
		c.spawnTimer(ctx, *event, picked.Id)
	}
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
