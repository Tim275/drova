package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"drova/services/payment-service/internal/events"
	"drova/services/payment-service/internal/infrastructure/stripe"
	"drova/services/payment-service/internal/service"
	"drova/services/payment-service/pkg/types"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"go.uber.org/zap"
)

func main() {
	log := logger.New(env.GetString("ENVIRONMENT", "development"))
	defer log.Sync()

	log.Infow("payment-service starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "payment-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
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
		log.Fatalw("STRIPE_SECRET_KEY is required")
	}

	kafkaBrokers := env.GetString("KAFKA_BROKERS", "localhost:9092")
	kafkaClient := messaging.NewKafka([]string{kafkaBrokers})
	defer kafkaClient.Close()

	processor := stripe.NewStripeClient(cfg)
	svc := service.NewPaymentService(processor)

	tripConsumer := events.NewTripConsumer(kafkaClient, svc)
	tripConsumer.Start(ctx)

	log.Infow("payment-service ready", "topic", "payment.cmd.create_session")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Infow("payment-service shutting down")
	cancel() // signals all consumers to stop
}
