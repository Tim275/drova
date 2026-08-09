package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
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

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
)

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, tracerErr := tracing.InitTracer(tracing.Config{
		ServiceName:           "payment-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	log := logger.New(envStr, "payment-service")
	defer log.Sync()

	if tracerErr != nil {
		log.Warnw("tracing init failed", zap.Error(tracerErr))
	}

	if err := runMigrations(
		env.GetString("DB_URL", ""),
		env.GetString("MIGRATIONS_PATH", "migrations"),
	); err != nil {
		log.Fatalw("migrations failed", zap.Error(err))
	}
	log.Infow("migrations complete")
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return
	}

	log.Infow("payment-service starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &types.PaymentConfig{
		StripeSecretKey: env.GetString("STRIPE_SECRET_KEY", ""),
		SuccessURL:      env.GetString("STRIPE_SUCCESS_URL", "http://localhost:8081/?payment=success"),
		CancelURL:       env.GetString("STRIPE_CANCEL_URL", "http://localhost:8081/?payment=cancel"),
		Currency:        env.GetString("STRIPE_CURRENCY", "eur"),
	}
	if cfg.StripeSecretKey == "" {
		log.Fatalw("STRIPE_SECRET_KEY is required")
	}

	poolURL := env.GetString("POOL_URL", env.GetString("DB_URL", ""))
	poolCfg, err := pgxpool.ParseConfig(stripMigrateParams(poolURL))
	if err != nil {
		log.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnIdleTime = 15 * time.Minute
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	kafkaClient := messaging.NewKafka(strings.Split(env.GetString("KAFKA_BROKERS", "localhost:9092"), ","))
	defer kafkaClient.Close()

	paymentStore := store.NewPaymentStore(db)
	processor := stripe.NewStripeClient(cfg)
	svc := service.NewPaymentService(processor)

	tripConsumer := events.NewTripConsumer(kafkaClient, svc, paymentStore, log)
	tripConsumer.Start(ctx)

	paymentConsumer := events.NewPaymentConsumer(kafkaClient, paymentStore, log)
	paymentConsumer.Start(ctx)
	kafkaClient.StartRetryConsumers(ctx, "payment-service")

	healthAddr := env.GetString("HEALTH_ADDR", ":8085")
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(pingCtx); err != nil {
			http.Error(w, "db: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	healthSrv := &http.Server{Addr: healthAddr, Handler: healthMux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warnw("health server error", "err", err)
		}
	}()

	log.Infow("payment-service ready", "health", healthAddr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Infow("payment-service shutting down")
	cancel()
	kafkaClient.Wait()
}

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
