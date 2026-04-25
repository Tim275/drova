package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"drova/shared/env"
	"drova/shared/messaging"
	"drova/shared/tracing"
)

var (
	httpAddr     = env.GetString("HTTP_ADDR", ":8081")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

var kafkaClient *messaging.Kafka

func main() {
	log.Println("Starting API Gateway")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "api-gateway",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Printf("warning: tracing init failed: %v", err)
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
	); err != nil {
		log.Printf("warning: ensure topics: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startNotificationConsumers(ctx)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", enableCORS(handleHealth))
	mux.HandleFunc("GET /config", enableCORS(handleConfig))
	mux.Handle("POST /trip/preview", tracing.WrapHandlerFunc(enableCORS(handleTripPreview), "POST /trip/preview"))
	mux.HandleFunc("OPTIONS /trip/preview", enableCORS(handleTripPreview))
	mux.Handle("POST /trip/start", tracing.WrapHandlerFunc(enableCORS(handleTripStart), "POST /trip/start"))
	mux.HandleFunc("OPTIONS /trip/start", enableCORS(handleTripStart))
	mux.Handle("/ws/drivers", tracing.WrapHandlerFunc(handleDriversWebSocket, "/ws/drivers"))
	mux.Handle("/ws/riders", tracing.WrapHandlerFunc(handleRidersWebSocket, "/ws/riders"))
	mux.Handle("POST /webhook/stripe", tracing.WrapHandlerFunc(handleStripeWebhook, "POST /webhook/stripe"))
	mux.Handle("/", http.FileServer(http.Dir("./frontend")))

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Server listening on %s", httpAddr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Printf("Error starting the server: %v", err)
	case sig := <-shutdown:
		log.Printf("Shutting down due to %v", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Could not stop the server gracefully: %v", err)
			server.Close()
		}
	}
}
