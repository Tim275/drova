package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/env"
	"drova/shared/messaging"
	pbt "drova/shared/proto/trip"
	pbu "drova/shared/proto/user"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const webhookDedupTTL = 24 * time.Hour

var userServiceProxy = func() *httputil.ReverseProxy {
	rawURL := env.GetString("USER_SERVICE_HTTP_URL", "http://user-service:8082")
	target, err := url.Parse(rawURL)
	if err != nil {
		panic("invalid USER_SERVICE_HTTP_URL: " + err.Error())
	}
	p := httputil.NewSingleHostReverseProxy(target)
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		appLog.Warnw("user-service proxy error", "err", err)
		http.Error(w, "user service unavailable", http.StatusBadGateway)
	}
	return p
}()

// handleRefresh proxies POST /v1/auth/refresh to user-service so that
// httpOnly cookies are preserved end-to-end without re-implementing token logic here.
func handleRefresh(w http.ResponseWriter, r *http.Request) {
	userServiceProxy.ServeHTTP(w, r)
}

var (
	mapboxPublicToken    = env.GetString("MAPBOX_PUBLIC_TOKEN", "")
	stripePublishableKey = env.GetString("STRIPE_PUBLIC_KEY", "")
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: map[string]string{"status": "ok", "service": "api-gateway"}})
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
	var businessErr error
	_, err := grpc_clients.TripBreaker.Execute(func() (interface{}, error) {
		preview, err := grpc_clients.TripClient.PreviewTrip(r.Context(), reqBody.toProto())
		if err != nil {
			if !grpc_clients.IsBreakerError(err) {
				businessErr = err
				return nil, nil
			}
			return nil, err
		}
		result = preview
		return nil, nil
	})
	if businessErr != nil {
		appLog.Warnw("preview trip: no route", zap.Error(businessErr))
		http.Error(w, "no route found for the given locations", http.StatusBadRequest)
		return
	}
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
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := grpc_clients.TripClient.GetTripsByUser(ctx, &pbt.GetTripsRequest{Id: userID})
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
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := grpc_clients.TripClient.GetTripsByDriver(ctx, &pbt.GetTripsRequest{Id: driverID})
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
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, err := grpc_clients.TripClient.RateTrip(ctx, &pbt.RateTripRequest{TripID: req.TripID, Rating: req.Rating})
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

	// Idempotency: Stripe retries webhooks on network failure.
	// Use Redis SET NX so only the first delivery publishes to Kafka.
	if gatewayRdb != nil {
		dedupKey := "drova:webhook:stripe:" + session.ID
		_, err := gatewayRdb.SetArgs(r.Context(), dedupKey, "1", redis.SetArgs{
			Mode: "NX",
			TTL:  webhookDedupTTL,
		}).Result()
		if errors.Is(err, redis.Nil) {
			// Key already existed — duplicate webhook, acknowledge safely.
			appLog.Infow("duplicate webhook ignored", "session", session.ID)
			w.WriteHeader(http.StatusOK)
			return
		}
		if err != nil {
			// Redis error — fail open to avoid silently dropping payments.
			appLog.Warnw("webhook dedup redis error", zap.Error(err))
		}
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

// ── User handlers (gRPC → HTTP translation) ──────────────────────────────────

func handleRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()

	if gatewayRdb != nil {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}
		key := "rl:register:" + ip
		count, err := gatewayRdb.Incr(r.Context(), key).Result()
		if err == nil {
			if count == 1 {
				gatewayRdb.Expire(r.Context(), key, 15*time.Minute)
			}
			if count > 10 {
				w.Header().Set("Retry-After", "900")
				http.Error(w, "too many registration attempts", http.StatusTooManyRequests)
				return
			}
		}
	}

	var req pbu.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var resp *pbu.RegisterResponse
	_, err := grpc_clients.UserBreaker.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := grpc_clients.UserClient.Register(ctx, &req)
		if err == nil {
			resp = res
		}
		return nil, err
	})
	if err != nil {
		grpcErrToHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": resp.Id, "email": resp.Email, "role": resp.Role,
		"message": "check your email to activate your account",
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	userServiceProxy.ServeHTTP(w, r)
}

func handleActivate(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	_, err := grpc_clients.UserBreaker.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, err := grpc_clients.UserClient.Activate(ctx, &pbu.ActivateRequest{Token: tok})
		return nil, err
	})
	if err != nil {
		grpcErrToHTTP(w, err)
		return
	}
	http.Redirect(w, r, "/?activated=true", http.StatusSeeOther)
}

func handleGetMe(w http.ResponseWriter, r *http.Request) {
	tokenStr := tokenFromRequest(r)
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var u *pbu.UserResponse
	_, err = grpc_clients.UserBreaker.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := grpc_clients.UserClient.GetUser(ctx, &pbu.GetUserRequest{UserId: claims.UserID})
		if err == nil {
			u = res
		}
		return nil, err
	})
	if err != nil {
		grpcErrToHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()

	tokenStr := tokenFromRequest(r)
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
		Phone       string `json:"phone"`
		Address     string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var u *pbu.UserResponse
	_, err = grpc_clients.UserBreaker.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := grpc_clients.UserClient.UpdateProfile(ctx, &pbu.UpdateProfileRequest{
			UserId:      claims.UserID,
			DisplayName: body.DisplayName,
			AvatarUrl:   body.AvatarURL,
			Phone:       body.Phone,
			Address:     body.Address,
		})
		if err == nil {
			u = res
		}
		return nil, err
	})
	if err != nil {
		grpcErrToHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	userServiceProxy.ServeHTTP(w, r)
}

func grpcErrToHTTP(w http.ResponseWriter, err error) {
	if err == gobreaker.ErrOpenState {
		http.Error(w, "user service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	st, _ := status.FromError(err)
	switch st.Code() {
	case codes.AlreadyExists:
		http.Error(w, st.Message(), http.StatusConflict)
	case codes.NotFound:
		http.Error(w, st.Message(), http.StatusNotFound)
	case codes.Unauthenticated:
		http.Error(w, st.Message(), http.StatusUnauthorized)
	case codes.InvalidArgument:
		http.Error(w, st.Message(), http.StatusBadRequest)
	default:
		appLog.Errorw("user grpc error", "code", st.Code(), "msg", st.Message())
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
