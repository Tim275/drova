package types

import "time"

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSuccess   PaymentStatus = "success"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCancelled PaymentStatus = "cancelled"
)

type PaymentIntent struct {
	ID              string    `json:"id"`
	TripID          string    `json:"trip_id"`
	UserID          string    `json:"user_id"`
	DriverID        string    `json:"driver_id"`
	Amount          int64     `json:"amount"`   // cents
	Currency        string    `json:"currency"` // "eur"
	StripeSessionID string    `json:"stripe_session_id"`
	CheckoutURL     string    `json:"checkout_url"`
	CreatedAt       time.Time `json:"created_at"`
}

type PaymentConfig struct {
	StripeSecretKey string
	SuccessURL      string
	CancelURL       string
	Currency        string
}
