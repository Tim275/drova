package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/env"
	"drova/shared/messaging"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

var (
	mapboxPublicToken     = env.GetString("MAPBOX_PUBLIC_TOKEN", "")
	stripePublishableKey = env.GetString("STRIPE_PUBLIC_KEY", "")
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: "i am alive"})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"mapboxToken":          mapboxPublicToken,
		"stripePublishableKey": stripePublishableKey,
	})
}

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB
	defer r.Body.Close()
	var reqBody startTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Printf("failed to connect to trip service: %v", err)
		http.Error(w, "failed to reach trip service", http.StatusInternalServerError)
		return
	}
	defer tripService.Close()

	trip, err := tripService.Client.CreateTrip(r.Context(), reqBody.toProto())
	if err != nil {
		log.Printf("failed to start trip: %v", err)
		http.Error(w, "failed to start trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, contracts.APIResponse{Data: map[string]string{"tripID": trip.GetTripID()}})
}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB
	defer r.Body.Close()
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Printf("failed to connect to trip service: %v", err)
		http.Error(w, "failed to reach trip service", http.StatusInternalServerError)
		return
	}
	defer tripService.Close()

	tripPreview, err := tripService.Client.PreviewTrip(r.Context(), reqBody.toProto())
	if err != nil {
		log.Printf("failed to preview trip: %v", err)
		http.Error(w, "failed to preview trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, contracts.APIResponse{Data: tripPreview})
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10) // 256 KB (Stripe payloads)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		log.Printf("STRIPE_WEBHOOK_KEY missing")
		http.Error(w, "webhook key not configured", http.StatusInternalServerError)
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body,
		r.Header.Get("Stripe-Signature"),
		webhookKey,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		log.Printf("Invalid webhook signature: %v", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Stripe event received: %s", event.Type)

	if event.Type != "checkout.session.completed" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		log.Printf("Failed to parse session: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	payload := messaging.PaymentStatusUpdate{
		TripID:   session.Metadata["trip_id"],
		UserID:   session.Metadata["user_id"],
		DriverID: session.Metadata["driver_id"],
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "marshal failed", http.StatusInternalServerError)
		return
	}

	msg := messaging.KafkaMessage{
		Type:    messaging.TopicPaymentSuccess,
		OwnerID: payload.UserID,
		Data:    payloadBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "marshal envelope failed", http.StatusInternalServerError)
		return
	}

	if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicPaymentSuccess, msgBytes); err != nil {
		log.Printf("Failed to publish payment.event.success: %v", err)
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Published payment.event.success: trip=%s user=%s", payload.TripID, payload.UserID)
	w.WriteHeader(http.StatusOK)
}
