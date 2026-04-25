package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"drova/services/payment-service/internal/events"
	"drova/services/payment-service/internal/infrastructure/stripe"
	"drova/services/payment-service/internal/service"
	"drova/services/payment-service/pkg/types"
	"drova/shared/env"
	"drova/shared/messaging"
	"drova/shared/tracing"
)

func main() {
	log.Println("Starting Payment Service")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "payment-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Printf("warning: tracing init failed: %v", err)
	} else {
		defer stopTracer(ctx)
	}

	cfg := &types.PaymentConfig{
		StripeSecretKey: env.GetString("STRIPE_SECRET_KEY", ""),
		SuccessURL:      env.GetString("STRIPE_SUCCESS_URL", "http://localhost:8081/?payment=success"),
		CancelURL:       env.GetString("STRIPE_CANCEL_URL", "http://localhost:8081/?payment=cancel"),
		Currency:        env.GetString("STRIPE_CURRENCY", "eur"),
	}
	if cfg.StripeSecretKey == "" {
		log.Fatal("STRIPE_SECRET_KEY is required")
	}

	kafkaBrokers := env.GetString("KAFKA_BROKERS", "localhost:9092")
	kafkaClient := messaging.NewKafka([]string{kafkaBrokers})
	defer kafkaClient.Close()

	processor := stripe.NewStripeClient(cfg)
	svc := service.NewPaymentService(processor)

	tripConsumer := events.NewTripConsumer(kafkaClient, svc)
	tripConsumer.Start(ctx)

	log.Println("Payment Service ready — listening for payment.cmd.create_session")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down payment service...")
}
