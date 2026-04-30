package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	httpAddr     = env.GetString("HTTP_ADDR", ":8081")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

var kafkaClient *messaging.Kafka
var appLog *zap.SugaredLogger
var gatewayRdb *redis.Client

func main() {
	appLog = logger.New(env.GetString("ENVIRONMENT", "development"))
	defer appLog.Sync()

	appLog.Infow("api-gateway starting")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "api-gateway",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	if err != nil {
		appLog.Warnw("tracing init failed", zap.Error(err))
	} else {
		defer stopTracer(context.Background())
	}

	kafkaClient = messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafkaClient.Close()

	if err := kafkaClient.EnsureTopics(
		messaging.TopicTripCreated,
		messaging.TopicDriverTripRequest,
		messaging.TopicDriverTripResponse,
		messaging.TopicTripDriverAssigned,
		messaging.TopicTripNoDriversFound,
		messaging.TopicDriverNotInterested,
		messaging.TopicDeadLetterQueue,
		messaging.TopicPaymentCreateSession,
		messaging.TopicPaymentSessionCreated,
		messaging.TopicPaymentSuccess,
		messaging.TopicTripCancelled,
		messaging.TopicTripDriverArrived,
		messaging.TopicTripInProgress,
		messaging.TopicTripCompleted,
		messaging.TopicDriverLocation,
	); err != nil {
		appLog.Warnw("ensure topics", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gatewayRdb = newRedisClient(
		env.GetString("REDIS_URL", "localhost:6379"),
		env.GetString("REDIS_PASSWORD", ""),
	)
	if gatewayRdb != nil {
		defer gatewayRdb.Close()
	}

	if err := grpc_clients.InitSharedClients(); err != nil {
		appLog.Warnw("grpc clients init", zap.Error(err))
	} else {
		defer grpc_clients.CloseSharedClients()
	}

	startNotificationConsumers(ctx)

	mux := http.NewServeMux()

	// Health / config
	mux.HandleFunc("GET /health", enableCORS(handleHealth))
	mux.HandleFunc("GET /readyz", enableCORS(handleReadyz(kafkaClient)))
	mux.HandleFunc("GET /config", enableCORS(handleConfig))

	// User endpoints (gRPC → HTTP translation)
	mux.HandleFunc("POST /v1/users/register", enableCORS(handleRegister))
	mux.HandleFunc("OPTIONS /v1/users/register", enableCORS(handleOptions))
	mux.HandleFunc("POST /v1/auth/token", enableCORS(handleLogin))
	mux.HandleFunc("OPTIONS /v1/auth/token", enableCORS(handleOptions))
	mux.HandleFunc("GET /v1/users/activate/{token}", handleActivate)
	mux.HandleFunc("GET /v1/users/me", enableCORS(requireAuth(handleGetMe)))
	mux.HandleFunc("PATCH /v1/users/profile", enableCORS(requireAuth(handleUpdateProfile)))
	mux.HandleFunc("OPTIONS /v1/users/profile", enableCORS(handleOptions))
	mux.HandleFunc("POST /v1/auth/logout", enableCORS(requireAuth(handleLogout)))
	mux.HandleFunc("OPTIONS /v1/auth/logout", enableCORS(handleOptions))

	// Trip endpoints
	mux.Handle("POST /trip/preview", tracing.WrapHandlerFunc(enableCORS(requireAuth(handleTripPreview)), "POST /trip/preview"))
	mux.HandleFunc("OPTIONS /trip/preview", enableCORS(handleOptions))
	mux.Handle("POST /trip/start", tracing.WrapHandlerFunc(enableCORS(requireAuth(handleTripStart)), "POST /trip/start"))
	mux.HandleFunc("OPTIONS /trip/start", enableCORS(handleOptions))
	mux.HandleFunc("GET /trips/history", enableCORS(requireAuth(handleTripHistory)))
	mux.HandleFunc("GET /trips/driver-history", enableCORS(handleDriverHistory))
	mux.Handle("POST /trip/rate", tracing.WrapHandlerFunc(enableCORS(requireAuth(handleTripRate)), "POST /trip/rate"))
	mux.HandleFunc("OPTIONS /trip/rate", enableCORS(handleOptions))

	// WebSocket + Stripe
	mux.Handle("/ws/drivers", tracing.WrapHandlerFunc(handleDriversWebSocket, "/ws/drivers"))
	mux.Handle("/ws/riders", tracing.WrapHandlerFunc(handleRidersWebSocket, "/ws/riders"))
	mux.Handle("POST /webhook/stripe", tracing.WrapHandlerFunc(handleStripeWebhook, "POST /webhook/stripe"))
	fs := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		fs.ServeHTTP(w, r)
	}))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           requestID(securityHeaders(gatewayRateLimit(gatewayRdb)(mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		appLog.Infow("api-gateway ready", "addr", httpAddr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		appLog.Errorw("server error", zap.Error(err))
	case sig := <-shutdown:
		appLog.Infow("shutting down", "signal", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			appLog.Errorw("graceful shutdown failed", zap.Error(err))
			server.Close()
		}
	}
}
