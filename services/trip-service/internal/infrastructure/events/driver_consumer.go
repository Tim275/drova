package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"drova/services/trip-service/internal/domain"
	"drova/shared/contracts"
	"drova/shared/messaging"
	pbd "drova/shared/proto/driver"
)

type DriverConsumer struct {
	kafka   *messaging.Kafka
	service domain.TripService
}

func NewDriverConsumer(kafka *messaging.Kafka, service domain.TripService) *DriverConsumer {
	return &DriverConsumer{kafka: kafka, service: service}
}

func (c *DriverConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicDriverTripResponse, "trip-service-driver-response", c.handleDriverResponse)
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

	log.Printf("Driver response: type=%s trip=%s", msg.Type, data.TripID)

	switch msg.Type {
	case contracts.DriverCmdTripAccept:
		return c.handleTripAccepted(ctx, data)
	case contracts.DriverCmdTripDecline:
		return c.handleTripDeclined(ctx, data)
	default:
		log.Printf("Unknown driver response type: %s", msg.Type)
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

	trip, err := c.service.GetTripByID(ctx, data.TripID)
	if err != nil || trip == nil {
		return fmt.Errorf("get trip for payment: %w", err)
	}

	paymentData := messaging.PaymentTripData{
		TripID:   data.TripID,
		UserID:   data.RiderID,
		DriverID: data.Driver.ID,
		Amount:   int64(trip.Fare.TotalPriceInCents),
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
		return fmt.Errorf("marshal payment envelope: %w", err)
	}
	if err := c.kafka.PublishMessage(ctx, messaging.TopicPaymentCreateSession, paymentPayload); err != nil {
		return fmt.Errorf("publish payment create_session: %w", err)
	}
	log.Printf("Published payment.cmd.create_session: trip=%s amount=%d eur", data.TripID, paymentData.Amount)

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
	log.Printf("Driver %s declined trip %s — re-searching via driver.event.not_interested", data.Driver.ID, data.TripID)

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
		TripID:      data.TripID,
		UserID:      data.RiderID,
		PackageSlug: trip.Fare.PackageSlug,
		Pickup:      pickup,
		Destination: dest,
		Route:       route,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := messaging.KafkaMessage{
		Type:    messaging.TopicDriverNotInterested,
		OwnerID: data.RiderID,
		Data:    eventData,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.kafka.PublishMessage(ctx, messaging.TopicDriverNotInterested, payload)
}
