package main

import (
	"context"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcHandler "drova/services/trip-service/internal/infrastructure/grpc"
	"drova/services/trip-service/internal/infrastructure/events"
	"drova/services/trip-service/internal/infrastructure/repository"
	"drova/services/trip-service/internal/service"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	grpcserver "google.golang.org/grpc"
)

var (
	grpcAddr     = env.GetString("GRPC_ADDR", ":9093")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

func main() {
	log := logger.New(env.GetString("ENVIRONMENT", "development"))
	defer log.Sync()

	log.Infow("trip-service starting")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "trip-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
	} else {
		defer stopTracer(context.Background())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kafka := messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafka.Close()

	poolCfg, err := pgxpool.ParseConfig(pgxURL(env.GetString("DB_URL", "")))
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

	if err := runMigrations(env.GetString("DB_URL", ""), env.GetString("MIGRATIONS_PATH", "services/trip-service/migrations")); err != nil {
		log.Fatalw("migrations", zap.Error(err))
	}

	publisher := events.NewTripEventPublisher(kafka, log)

	pgRepo := repository.NewPostgresRepository(db)
	svc := service.NewService(pgRepo)

	driverConsumer := events.NewDriverConsumer(kafka, svc, log)
	paymentConsumer := events.NewPaymentConsumer(kafka, svc, log)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalw("listen failed", zap.Error(err))
	}

	grpcSrv := grpcserver.NewServer(tracing.WithTracingInterceptors()...)
	grpcHandler.NewGRPCHandler(grpcSrv, svc, publisher, log)

	driverConsumer.Start(ctx)
	paymentConsumer.Start(ctx)

	go func() {
		log.Infow("grpc server ready", "addr", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Errorw("grpc server error", zap.Error(err))
			cancel()
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case <-shutdown:
		log.Infow("trip-service shutting down")
	case <-ctx.Done():
	}

	grpcSrv.GracefulStop()
}

func pgxURL(dbURL string) string {
	u, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}
	q := u.Query()
	q.Del("x-migrations-table")
	u.RawQuery = q.Encode()
	return u.String()
}
