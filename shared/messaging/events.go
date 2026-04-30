package messaging

import (
	"encoding/json"
	"time"
)

type KafkaMessage struct {
	Type    string          `json:"type"`
	OwnerID string          `json:"owner_id"`
	Data    json.RawMessage `json:"data"`
}

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type TripCreatedEvent struct {
	TripID           string       `json:"trip_id"`
	UserID           string       `json:"user_id"`
	RiderName        string       `json:"rider_name,omitempty"`
	RiderAvatar      string       `json:"rider_avatar,omitempty"`
	PackageSlug      string       `json:"package_slug"`
	PickupGeohash    string       `json:"pickup_geohash"`
	Pickup           Coordinate   `json:"pickup"`
	Destination      Coordinate   `json:"destination"`
	Route            []Coordinate `json:"route"`
	ExcludeDriverIDs []string     `json:"exclude_driver_ids,omitempty"`
}

type DriverInfo struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ProfilePicture string  `json:"profile_picture"`
	CarPlate       string  `json:"car_plate"`
	Lat            float64 `json:"lat,omitempty"`
	Lng            float64 `json:"lng,omitempty"`
}

type DriverTripAcceptData struct {
	TripID  string `json:"trip_id"`
	RiderID string `json:"rider_id"`
}

type DriverTripResponseData struct {
	Driver  DriverInfo `json:"driver"`
	TripID  string     `json:"trip_id"`
	RiderID string     `json:"rider_id"`
}

type PaymentTripData struct {
	TripID   string `json:"trip_id"`
	UserID   string `json:"user_id"`
	DriverID string `json:"driver_id"`
	Amount   int64  `json:"amount"`   // cents
	Currency string `json:"currency"` // "eur"
}

type PaymentSessionCreated struct {
	TripID      string `json:"trip_id"`
	SessionID   string `json:"session_id"`
	CheckoutURL string `json:"checkout_url"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

type PaymentStatusUpdate struct {
	TripID          string `json:"trip_id"`
	UserID          string `json:"user_id"`
	DriverID        string `json:"driver_id"`
	StripeSessionID string `json:"stripe_session_id"`
}

type DriverLocationData struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RiderID  string  `json:"rider_id"`
}

type TripStatusEvent struct {
	TripID   string `json:"trip_id"`
	RiderID  string `json:"rider_id"`
	DriverID string `json:"driver_id"`
}

type TripCancelledEvent struct {
	TripID   string `json:"trip_id"`
	RiderID  string `json:"rider_id"`
	DriverID string `json:"driver_id"`
}

type DLQMessage struct {
	OriginalTopic string          `json:"original_topic"`
	OriginalKey   string          `json:"original_key"`
	FailureReason string          `json:"failure_reason"`
	RetryCount    int             `json:"retry_count"`
	FailedAt      time.Time       `json:"failed_at"`
	Payload       json.RawMessage `json:"payload"`
}
