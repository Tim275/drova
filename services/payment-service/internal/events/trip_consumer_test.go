package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"drova/services/payment-service/internal/domain"
	"drova/services/payment-service/pkg/types"
	"drova/shared/messaging"

	"go.uber.org/zap"
)

// --- mocks ---

type mockStore struct {
	existing   *domain.Payment
	saveCalled bool
}

func (m *mockStore) GetByTripID(_ context.Context, _ string) (*domain.Payment, error) {
	if m.existing != nil {
		return m.existing, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStore) Save(_ context.Context, _ *domain.Payment) error {
	m.saveCalled = true
	return nil
}

func (m *mockStore) MarkSuccess(_ context.Context, _ string, _ time.Time) error { return nil }

type mockService struct {
	called bool
}

func (m *mockService) CreatePaymentSession(_ context.Context, _, _, _ string, _ int64, _ string) (*types.PaymentIntent, error) {
	m.called = true
	return &types.PaymentIntent{
		TripID:          "trip-123",
		StripeSessionID: "sess_new",
		CheckoutURL:     "https://checkout.stripe.com/pay/sess_new",
		Amount:          1500,
		Currency:        "eur",
	}, nil
}

// --- helpers ---

func buildRawMessage(tripID, userID string, amount int64) []byte {
	tripData := messaging.PaymentTripData{
		TripID:   tripID,
		UserID:   userID,
		DriverID: "drv-1",
		Amount:   amount,
		Currency: "eur",
	}
	dataBytes, _ := json.Marshal(tripData)
	env := messaging.KafkaMessage{
		Type:    messaging.TopicPaymentCreateSession,
		OwnerID: userID,
		Data:    dataBytes,
	}
	raw, _ := json.Marshal(env)
	return raw
}

// --- tests ---

// Kafka redelivers the same trip.event.completed twice.
// The second call must not create a second Stripe session.
func TestHandleCreateSession_Idempotency(t *testing.T) {
	store := &mockStore{
		existing: &domain.Payment{
			TripID:          "trip-123",
			StripeSessionID: "sess_existing",
		},
	}
	svc := &mockService{}

	c := &TripConsumer{
		kafka:   nil, // not reached in idempotency path
		service: svc,
		store:   store,
		log:     zap.NewNop().Sugar(),
	}

	err := c.handleCreateSession(context.Background(), buildRawMessage("trip-123", "42", 1500))

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if svc.called {
		t.Error("Stripe CreatePaymentSession must not be called for a duplicate trip")
	}
	if store.saveCalled {
		t.Error("Save must not be called for a duplicate trip")
	}
}

// First-time message: Stripe session is created and payment is saved.
// Uses a mock kafka=nil so we only test up to the publish step;
// the publish itself would need a real broker (integration test territory).
func TestHandleCreateSession_FirstTime_CallsStripe(t *testing.T) {
	store := &mockStore{} // existing == nil → GetByTripID returns error → no duplicate
	svc := &mockService{}

	c := &TripConsumer{
		kafka:   nil,
		service: svc,
		store:   store,
		log:     zap.NewNop().Sugar(),
	}

	// We expect an error here because kafka.PublishMessage on nil panics/errors,
	// but we can verify Stripe was called and Save was called BEFORE the publish.
	// Use recover to catch the nil-pointer from kafka and still assert the mocks.
	func() {
		defer func() { recover() }() //nolint:errcheck
		_ = c.handleCreateSession(context.Background(), buildRawMessage("trip-456", "99", 2000))
	}()

	if !svc.called {
		t.Error("Stripe CreatePaymentSession must be called for a new trip")
	}
	if !store.saveCalled {
		t.Error("Save must be called after Stripe session is created")
	}
}
