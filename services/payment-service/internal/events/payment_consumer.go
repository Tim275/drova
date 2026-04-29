package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"drova/services/payment-service/internal/domain"
	"drova/shared/messaging"

	"go.uber.org/zap"
)

type PaymentConsumer struct {
	kafka *messaging.Kafka
	store domain.PaymentStore
	log   *zap.SugaredLogger
}

func NewPaymentConsumer(kafka *messaging.Kafka, store domain.PaymentStore, log *zap.SugaredLogger) *PaymentConsumer {
	return &PaymentConsumer{kafka: kafka, store: store, log: log}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	go c.kafka.ConsumeMessages(ctx, messaging.TopicPaymentSuccess, "payment-service-success", c.handleSuccess)
}

func (c *PaymentConsumer) handleSuccess(ctx context.Context, raw []byte) error {
	var env messaging.KafkaMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}

	var data messaging.PaymentStatusUpdate
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("unmarshal payment status update: %w", err)
	}

	if data.StripeSessionID == "" {
		c.log.Warnw("payment.event.success missing stripe_session_id", "trip", data.TripID)
		return nil
	}

	if err := c.store.MarkSuccess(ctx, data.StripeSessionID, time.Now().UTC()); err != nil {
		return fmt.Errorf("mark payment success trip=%s: %w", data.TripID, err)
	}

	c.log.Infow("payment marked success", "trip", data.TripID, "session", data.StripeSessionID)
	return nil
}
