package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/env"
	"drova/shared/messaging"
	pb "drova/shared/proto/trip"

	"github.com/sony/gobreaker"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"go.uber.org/zap"
)

var (
	mapboxPublicToken    = env.GetString("MAPBOX_PUBLIC_TOKEN", "")
	stripePublishableKey = env.GetString("STRIPE_PUBLIC_KEY", "")
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: "i am alive"})
}

func handleReadyz(kafka interface{ Ping(ctx context.Context) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := kafka.Ping(r.Context()); err != nil {
			appLog.Warnw("readyz: kafka unreachable", "err", err)
			http.Error(w, "kafka unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, contracts.APIResponse{Data: "ready"})
	}
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"mapboxToken":          mapboxPublicToken,
		"stripePublishableKey": stripePublishableKey,
	})
}

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var reqBody startTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	var tripID string
	_, err := grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		trip, err := grpc_clients.TripClient.CreateTrip(r.Context(), reqBody.toProto())
		if err != nil {
			return nil, err
		}
		tripID = trip.GetTripID()
		return nil, nil
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			appLog.Warnw("trip service circuit open")
			http.Error(w, "trip service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		appLog.Errorw("create trip", zap.Error(err))
		http.Error(w, "failed to start trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, contracts.APIResponse{Data: map[string]string{"tripID": tripID}})
}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	var result interface{}
	_, err := grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		preview, err := grpc_clients.TripClient.PreviewTrip(r.Context(), reqBody.toProto())
		if err != nil {
			return nil, err
		}
		result = preview
		return nil, nil
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			appLog.Warnw("trip service circuit open")
			http.Error(w, "trip service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		appLog.Errorw("preview trip", zap.Error(err))
		http.Error(w, "failed to preview trip", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, contracts.APIResponse{Data: result})
}

func handleTripHistory(w http.ResponseWriter, r *http.Request) {
	tokenStr := tokenFromRequest(r)
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID := fmt.Sprintf("%d", claims.UserID)

	var resp interface{}
	_, err = grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		client, err := grpc_clients.NewTripServiceClient()
		if err != nil {
			return nil, err
		}
		defer client.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := client.Client.GetTripsByUser(ctx, &pb.GetTripsRequest{Id: userID})
		if err != nil {
			return nil, err
		}
		resp = res.GetTrips()
		return nil, nil
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			http.Error(w, "trip service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		appLog.Errorw("trip history", zap.Error(err))
		http.Error(w, "trip service unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleDriverHistory(w http.ResponseWriter, r *http.Request) {
	driverID := r.URL.Query().Get("driverID")
	if driverID == "" {
		http.Error(w, "driverID required", http.StatusBadRequest)
		return
	}

	var resp interface{}
	_, err := grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		client, err := grpc_clients.NewTripServiceClient()
		if err != nil {
			return nil, err
		}
		defer client.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := client.Client.GetTripsByDriver(ctx, &pb.GetTripsRequest{Id: driverID})
		if err != nil {
			return nil, err
		}
		resp = res.GetTrips()
		return nil, nil
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			http.Error(w, "trip service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		appLog.Errorw("driver history", zap.Error(err))
		http.Error(w, "trip service unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleTripRate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var req struct {
		TripID string `json:"trip_id"`
		Rating int32  `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	_, err := grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		client, err := grpc_clients.NewTripServiceClient()
		if err != nil {
			return nil, err
		}
		defer client.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, err = client.Client.RateTrip(ctx, &pb.RateTripRequest{TripID: req.TripID, Rating: req.Rating})
		return nil, err
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			http.Error(w, "trip service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		appLog.Errorw("trip rate", zap.Error(err))
		http.Error(w, "trip service unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		appLog.Errorw("STRIPE_WEBHOOK_KEY missing")
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
		appLog.Warnw("invalid webhook signature", zap.Error(err))
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	appLog.Infow("stripe event", "type", event.Type)

	if event.Type != "checkout.session.completed" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		appLog.Errorw("parse stripe session", zap.Error(err))
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	payload := messaging.PaymentStatusUpdate{
		TripID:          session.Metadata["trip_id"],
		UserID:          session.Metadata["user_id"],
		DriverID:        session.Metadata["driver_id"],
		StripeSessionID: session.ID,
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
		appLog.Errorw("publish payment success", zap.Error(err))
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}

	appLog.Infow("payment success published", "trip", payload.TripID, "user", payload.UserID)
	w.WriteHeader(http.StatusOK)
}
