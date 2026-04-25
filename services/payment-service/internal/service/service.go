package service

import (
	"context"
	"fmt"
	"time"

	"drova/services/payment-service/internal/domain"
	"drova/services/payment-service/pkg/types"

	"github.com/google/uuid"
)

type paymentService struct {
	processor domain.PaymentProcessor
}

func NewPaymentService(processor domain.PaymentProcessor) domain.Service {
	return &paymentService{processor: processor}
}

func (s *paymentService) CreatePaymentSession(
	ctx context.Context,
	tripID, userID, driverID string,
	amount int64,
	currency string,
) (*types.PaymentIntent, error) {
	metadata := map[string]string{
		"trip_id":   tripID,
		"user_id":   userID,
		"driver_id": driverID,
	}

	sessionID, checkoutURL, err := s.processor.CreatePaymentSession(ctx, amount, currency, metadata)
	if err != nil {
		return nil, fmt.Errorf("create payment session: %w", err)
	}

	return &types.PaymentIntent{
		ID:              uuid.New().String(),
		TripID:          tripID,
		UserID:          userID,
		DriverID:        driverID,
		Amount:          amount,
		Currency:        currency,
		StripeSessionID: sessionID,
		CheckoutURL:     checkoutURL,
		CreatedAt:       time.Now().UTC(),
	}, nil
}
