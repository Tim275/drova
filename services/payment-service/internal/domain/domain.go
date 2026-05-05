package domain

import (
	"context"
	"time"

	"drova/services/payment-service/pkg/types"
)

type Service interface {
	CreatePaymentSession(ctx context.Context, tripID, userID, driverID string, amount int64, currency string) (*types.PaymentIntent, error)
}

type PaymentProcessor interface {
	CreatePaymentSession(ctx context.Context, amount int64, currency string, metadata map[string]string) (sessionID, checkoutURL string, err error)
}

type Payment struct {
	ID                     int64
	TripID                 string
	UserID                 int64
	DriverID               string
	AmountCents            int64
	Currency               string
	Status                 types.PaymentStatus
	StripeSessionID        string
	StripePaymentIntentID  string
	DriverPayoutCents      int64
	PlatformFeeCents       int64
	RefundID               string
	CreatedAt              time.Time
	PaidAt                 *time.Time
	RefundedAt             *time.Time
}

type PaymentStore interface {
	Save(ctx context.Context, p *Payment) error
	GetByTripID(ctx context.Context, tripID string) (*Payment, error)
	MarkSuccess(ctx context.Context, stripeSessionID string, paidAt time.Time) error
}
