package events

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"drova/services/trip-service/internal/domain"
	"drova/shared/contracts"
	"drova/shared/messaging"
	pbd "drova/shared/proto/driver"

	"go.uber.org/zap"
)

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

type DriverConsumer struct {
	kafka   *messaging.Kafka
	service domain.TripService
	log     *zap.SugaredLogger
}

func NewDriverConsumer(kafka *messaging.Kafka, service domain.TripService, log *zap.SugaredLogger) *DriverConsumer {
	return &DriverConsumer{kafka: kafka, service: service, log: log}
}

func (c *DriverConsumer) Start(ctx context.Context) {
	c.kafka.ConsumeMessages(ctx, messaging.TopicDriverTripResponse, "trip-service-driver-response", c.handleDriverResponse)
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripCompleted, "trip-service-completed", c.handleTripCompleted)
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripCancelled, "trip-service-cancelled", c.handleTripCancelled)
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripDriverArrived, "trip-service-arrived", c.handleStatusUpdate("driver_arrived"))
	c.kafka.ConsumeMessages(ctx, messaging.TopicTripInProgress, "trip-service-in-progress", c.handleStatusUpdate("in_progress"))
}

func (c *DriverConsumer) handleDriverResponse(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("unmarshal kafka message: %w", err)
	}

	var data messaging.DriverTripResponseData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return fmt.Errorf("unmarshal response data: %w", err)
	}

	c.log.Infow("driver response", "type", msg.Type, "trip", data.TripID)

	switch msg.Type {
	case contracts.DriverCmdTripAccept:
		return c.handleTripAccepted(ctx, data)
	case contracts.DriverCmdTripDecline:
		return c.handleTripDeclined(ctx, data)
	default:
		c.log.Warnw("unknown driver response type", "type", msg.Type)
	}
	return nil
}

func (c *DriverConsumer) handleTripAccepted(ctx context.Context, data messaging.DriverTripResponseData) error {
	driver := &pbd.Driver{
		Id:             data.Driver.ID,
		Name:           data.Driver.Name,
		ProfilePicture: data.Driver.ProfilePicture,
		CarPlate:       data.Driver.CarPlate,
	}

	if err := c.service.UpdateTrip(ctx, data.TripID, "accepted", driver); err != nil {
		return fmt.Errorf("update trip: %w", err)
	}

	driverData, err := json.Marshal(data.Driver)
	if err != nil {
		return fmt.Errorf("marshal driver: %w", err)
	}

	notification := messaging.KafkaMessage{
		Type:    contracts.TripEventDriverAssigned,
		OwnerID: data.RiderID,
		Data:    driverData,
	}
	notifyPayload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	return c.kafka.PublishMessage(ctx, messaging.TopicTripDriverAssigned, notifyPayload)
}

func (c *DriverConsumer) handleTripDeclined(ctx context.Context, data messaging.DriverTripResponseData) error {
	c.log.Infow("driver declined", "driver", data.Driver.ID, "trip", data.TripID)

	trip, err := c.service.GetTripByID(ctx, data.TripID)
	if err != nil || trip == nil {
		return fmt.Errorf("get trip: %w", err)
	}

	rawCoords := trip.Fare.Route.Routes[0].Geometry.Coordinates
	pickup := messaging.Coordinate{Lat: rawCoords[0][1], Lng: rawCoords[0][0]}
	dest := messaging.Coordinate{Lat: rawCoords[len(rawCoords)-1][1], Lng: rawCoords[len(rawCoords)-1][0]}
	route := make([]messaging.Coordinate, len(rawCoords))
	for i, p := range rawCoords {
		route[i] = messaging.Coordinate{Lat: p[1], Lng: p[0]}
	}

	event := messaging.TripCreatedEvent{
		TripID:           data.TripID,
		UserID:           data.RiderID,
		PackageSlug:      trip.Fare.PackageSlug,
		Pickup:           pickup,
		Destination:      dest,
		Route:            route,
		ExcludeDriverIDs: []string{data.Driver.ID},
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal retry event: %w", err)
	}
	msg := messaging.KafkaMessage{
		Type:    messaging.TopicDriverNotInterested,
		OwnerID: data.RiderID,
		Data:    eventData,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal retry message: %w", err)
	}
	return c.kafka.PublishMessage(ctx, messaging.TopicDriverNotInterested, payload)
}

func (c *DriverConsumer) handleTripCompleted(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	var data messaging.TripStatusEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return fmt.Errorf("unmarshal status event: %w", err)
	}

	if err := c.service.UpdateTrip(ctx, data.TripID, "completed", nil); err != nil {
		c.log.Warnw("update completed trip", "trip", data.TripID, zap.Error(err))
	}

	trip, err := c.service.GetTripByID(ctx, data.TripID)
	if err != nil || trip == nil {
		return fmt.Errorf("get trip for payment: %w", err)
	}
	if trip.Fare == nil {
		c.log.Warnw("trip has no fare, skipping payment", "trip", data.TripID)
		return nil
	}

	amount := int64(trip.Fare.TotalPriceInCents)
	if data.ActualLat != 0 && data.ActualLng != 0 && trip.Fare.Route != nil && len(trip.Fare.Route.Routes) > 0 {
		r := trip.Fare.Route.Routes[0]
		if len(r.Geometry.Coordinates) > 0 {
			pickupLng, pickupLat := r.Geometry.Coordinates[0][0], r.Geometry.Coordinates[0][1]
			actualKm := haversineKm(pickupLat, pickupLng, data.ActualLat, data.ActualLng)
			plannedKm := r.Distance / 1000
			if plannedKm > 0 && actualKm < plannedKm*0.95 {
				ratio := actualKm / plannedKm
				if ratio < 0.1 {
					ratio = 0.1
				}
				amount = int64(float64(trip.Fare.TotalPriceInCents) * ratio)
			}
		}
	}

	paymentData := messaging.PaymentTripData{
		TripID:   data.TripID,
		UserID:   data.RiderID,
		DriverID: data.DriverID,
		Amount:   amount,
		Currency: "eur",
	}
	paymentBytes, err := json.Marshal(paymentData)
	if err != nil {
		return fmt.Errorf("marshal payment data: %w", err)
	}
	paymentMsg := messaging.KafkaMessage{
		Type:    messaging.TopicPaymentCreateSession,
		OwnerID: data.RiderID,
		Data:    paymentBytes,
	}
	paymentPayload, err := json.Marshal(paymentMsg)
	if err != nil {
		return fmt.Errorf("marshal payment message: %w", err)
	}
	if err := c.kafka.PublishMessage(ctx, messaging.TopicPaymentCreateSession, paymentPayload); err != nil {
		return fmt.Errorf("publish payment create_session: %w", err)
	}
	c.log.Infow("payment session requested", "trip", data.TripID, "amount", paymentData.Amount)
	return nil
}

func (c *DriverConsumer) handleTripCancelled(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil
	}
	var data messaging.TripCancelledEvent
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		return nil
	}
	return c.service.CancelTrip(ctx, data.TripID)
}

func (c *DriverConsumer) handleStatusUpdate(status string) func(context.Context, []byte) error {
	return func(ctx context.Context, payload []byte) error {
		var msg messaging.KafkaMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil
		}
		var data messaging.TripStatusEvent
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return nil
		}
		return c.service.UpdateTrip(ctx, data.TripID, status, nil)
	}
}
