package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"drova/services/trip-service/internal/domain"
	"drova/shared/messaging"
)

type PaymentConsumer struct {
	kafka   *messaging.Kafka
	service domain.TripService
}

func NewPaymentConsumer(kafka *messaging.Kafka, service domain.TripService) *PaymentConsumer {
	return &PaymentConsumer{kafka: kafka, service: service}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicPaymentSuccess, "trip-service-payment-success", c.handlePaymentSuccess)
}

func (c *PaymentConsumer) handlePaymentSuccess(ctx context.Context, raw []byte) error {
	var env messaging.KafkaMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var payload messaging.PaymentStatusUpdate
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return fmt.Errorf("unmarshal payment status: %w", err)
	}

	log.Printf("Trip paid: trip=%s user=%s", payload.TripID, payload.UserID)

	return c.service.UpdateTrip(ctx, payload.TripID, "paid", nil)
}
