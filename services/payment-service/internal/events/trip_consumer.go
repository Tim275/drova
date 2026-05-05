package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"drova/services/payment-service/internal/domain"
	"drova/services/payment-service/pkg/types"
	"drova/shared/messaging"

	"go.uber.org/zap"
)

type TripConsumer struct {
	kafka   *messaging.Kafka
	service domain.Service
	store   domain.PaymentStore
	log     *zap.SugaredLogger
}

func NewTripConsumer(kafka *messaging.Kafka, service domain.Service, store domain.PaymentStore, log *zap.SugaredLogger) *TripConsumer {
	return &TripConsumer{kafka: kafka, service: service, store: store, log: log}
}

func (c *TripConsumer) Start(ctx context.Context) {
	c.kafka.ConsumeMessages(ctx, messaging.TopicPaymentCreateSession, "payment-service-create", c.handleCreateSession)
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

	// Idempotency guard: Kafka at-least-once means this can fire twice for the same trip.
	// If a session already exists, republish the checkout URL so the rider still gets it.
	if existing, err := c.store.GetByTripID(ctx, data.TripID); err == nil && existing != nil {
		c.log.Infow("payment session already exists, skipping duplicate", "trip", data.TripID, "session", existing.StripeSessionID)
		return nil
	}

	c.log.Infow("creating stripe session", "trip", data.TripID, "user", data.UserID, "amount", data.Amount, "currency", data.Currency)
	intent, err := c.service.CreatePaymentSession(ctx, data.TripID, data.UserID, data.DriverID, data.Amount, data.Currency)
	if err != nil {
		return fmt.Errorf("create payment session: %w", err)
	}
	c.log.Infow("stripe session created", "session", intent.StripeSessionID, "trip", data.TripID)

	userID, _ := strconv.ParseInt(data.UserID, 10, 64)
	payment := &domain.Payment{
		TripID:          intent.TripID,
		UserID:          userID,
		DriverID:        intent.DriverID,
		AmountCents:     intent.Amount,
		Currency:        intent.Currency,
		Status:          types.PaymentStatusPending,
		StripeSessionID: intent.StripeSessionID,
		CreatedAt:       time.Now().UTC(),
	}
	if err := c.store.Save(ctx, payment); err != nil {
		c.log.Warnw("failed to save pending payment", "trip", data.TripID, "error", err)
	}

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
		OwnerID: data.UserID,
		Data:    createdBytes,
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	return c.kafka.PublishMessage(ctx, messaging.TopicPaymentSessionCreated, payload)
}
