package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drova/services/payment-service/internal/events"
	"drova/services/payment-service/internal/infrastructure/stripe"
	"drova/services/payment-service/internal/service"
	"drova/services/payment-service/internal/store"
	"drova/services/payment-service/pkg/types"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	log := logger.New(env.GetString("ENVIRONMENT", "development"))
	defer log.Sync()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := runMigrations(
			env.GetString("DB_URL", ""),
			env.GetString("MIGRATIONS_PATH", "migrations"),
		); err != nil {
			log.Fatalw("migrations failed", fmt.Sprintf("%v", err))
		}
		log.Infow("migrations complete")
		return
	}

	log.Infow("payment-service starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "payment-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
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

	poolCfg, err := pgxpool.ParseConfig(stripMigrateParams(env.GetString("DB_URL", "")))
	if err != nil {
		log.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnIdleTime = 15 * time.Minute

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	migrationsPath := env.GetString("MIGRATIONS_PATH", "services/payment-service/migrations")
	if err := runMigrations(env.GetString("DB_URL", ""), migrationsPath); err != nil {
		log.Fatalw("migrations", zap.Error(err))
	}

	kafkaBrokers := env.GetString("KAFKA_BROKERS", "localhost:9092")
	kafkaClient := messaging.NewKafka([]string{kafkaBrokers})
	defer kafkaClient.Close()

	paymentStore := store.NewPaymentStore(db)
	processor := stripe.NewStripeClient(cfg)
	svc := service.NewPaymentService(processor)

	tripConsumer := events.NewTripConsumer(kafkaClient, svc, paymentStore, log)
	tripConsumer.Start(ctx)

	paymentConsumer := events.NewPaymentConsumer(kafkaClient, paymentStore, log)
	paymentConsumer.Start(ctx)

	log.Infow("payment-service ready")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Infow("payment-service shutting down")
	cancel()
}

// stripMigrateParams removes golang-migrate specific query parameters (x-* prefix)
// that pgxpool would forward as server parameters, causing a PostgreSQL FATAL error.
func stripMigrateParams(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for k := range q {
		if len(k) > 2 && k[:2] == "x-" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
