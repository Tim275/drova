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
	"drova/shared/schema"
	"drova/shared/tracing"

	"github.com/hamba/avro/v2"
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
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "api-gateway",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	appLog = logger.New(envStr, "api-gateway")
	defer appLog.Sync()

	if err != nil {
		appLog.Warnw("tracing init failed", zap.Error(err))
	}

	appLog.Infow("api-gateway starting")

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

	kafkaClient.RegisterSchemas(context.Background(), map[string]avro.Schema{
		"trip-created-value":          schema.SchemaTripCreated,
		"trip-status-event-value":     schema.SchemaTripStatus,
		"driver-trip-request-value":   schema.SchemaDriverTripRequest,
		"payment-status-update-value": schema.SchemaPaymentStatusUpdate,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gatewayRdb = newRedisClient(
		env.GetString("REDIS_URL", "localhost:6379"),
		env.GetString("REDIS_PASSWORD", ""),
	)
	if gatewayRdb != nil {
		defer gatewayRdb.Close()
	}

	grpc_clients.InitBreakers(appLog)
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
	mux.HandleFunc("POST /v1/auth/refresh", enableCORS(handleRefresh))
	mux.HandleFunc("OPTIONS /v1/auth/refresh", enableCORS(handleOptions))
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
	mux.HandleFunc("GET /trips/driver-history", enableCORS(requireAuth(handleDriverHistory)))
	mux.Handle("POST /trip/rate", tracing.WrapHandlerFunc(enableCORS(requireAuth(handleTripRate)), "POST /trip/rate"))
	mux.HandleFunc("OPTIONS /trip/rate", enableCORS(handleOptions))

	// WebSocket + Stripe
	mux.Handle("/ws/drivers", tracing.WrapHandlerFunc(handleDriversWebSocket, "/ws/drivers"))
	mux.Handle("/ws/riders", tracing.WrapHandlerFunc(handleRidersWebSocket, "/ws/riders"))
	mux.Handle("POST /webhook/stripe", tracing.WrapHandlerFunc(handleStripeWebhook, "POST /webhook/stripe"))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           tracing.WrapHandler(requestID(securityHeaders(mux)), "api-gateway"),
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
		kafkaClient.Wait()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			appLog.Errorw("graceful shutdown failed", zap.Error(err))
			server.Close()
		}
	}
}
