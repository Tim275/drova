package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"drova/services/payment-service/internal/domain"
	"drova/shared/messaging"
)

type TripConsumer struct {
	kafka   *messaging.Kafka
	service domain.Service
}

func NewTripConsumer(kafka *messaging.Kafka, service domain.Service) *TripConsumer {
	return &TripConsumer{kafka: kafka, service: service}
}

func (c *TripConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicPaymentCreateSession, "payment-service-create", c.handleCreateSession)
}

func (c *TripConsumer) handleCreateSession(ctx context.Context, raw []byte) error {
	var env messaging.KafkaMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var data messaging.PaymentTripData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("unmarshal payment trip data: %w", err)
	}

	log.Printf("Creating Stripe session for trip=%s user=%s amount=%d %s", data.TripID, data.UserID, data.Amount, data.Currency)

	intent, err := c.service.CreatePaymentSession(ctx, data.TripID, data.UserID, data.DriverID, data.Amount, data.Currency)
	if err != nil {
		return fmt.Errorf("create payment session: %w", err)
	}

	log.Printf("Stripe session created: %s", intent.StripeSessionID)

	created := messaging.PaymentSessionCreated{
		TripID:      intent.TripID,
		SessionID:   intent.StripeSessionID,
		CheckoutURL: intent.CheckoutURL,
		Amount:      intent.Amount,
		Currency:    intent.Currency,
	}
	createdBytes, err := json.Marshal(created)
	if err != nil {
		return fmt.Errorf("marshal created event: %w", err)
	}

	notification := messaging.KafkaMessage{
		Type:    messaging.TopicPaymentSessionCreated,
		OwnerID: data.UserID, // Rider soll benachrichtigt werden
		Data:    createdBytes,
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	return c.kafka.PublishMessage(ctx, messaging.TopicPaymentSessionCreated, payload)
}
