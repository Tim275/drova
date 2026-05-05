package events

import (
	"context"
	"encoding/json"
	"fmt"

	"drova/services/trip-service/internal/domain"
	"drova/shared/messaging"

	"go.uber.org/zap"
)

type PaymentConsumer struct {
	kafka   *messaging.Kafka
	service domain.TripService
	log     *zap.SugaredLogger
}

func NewPaymentConsumer(kafka *messaging.Kafka, service domain.TripService, log *zap.SugaredLogger) *PaymentConsumer {
	return &PaymentConsumer{kafka: kafka, service: service, log: log}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	c.kafka.ConsumeMessages(ctx, messaging.TopicPaymentSuccess, "trip-service-payment-success", c.handlePaymentSuccess)
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

	c.log.Infow("trip paid", "trip", payload.TripID, "user", payload.UserID)

	return c.service.UpdateTrip(ctx, payload.TripID, "paid", nil)
}
