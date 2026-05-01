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

	"drova/services/driver-service/store"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	grpcserver "google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	grpcAddr     = env.GetString("GRPC_ADDR", ":9092")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

var appLog *zap.SugaredLogger

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "driver-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	appLog = logger.New(envStr, "driver-service")
	defer appLog.Sync()

	if err != nil {
		appLog.Warnw("tracing init failed", zap.Error(err))
	}

	appLog.Infow("driver-service starting")

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := runMigrations(env.GetString("DB_URL", ""), env.GetString("MIGRATIONS_PATH", "services/driver-service/migrations")); err != nil {
			appLog.Fatalw("migrations failed", zap.Error(err))
		}
		appLog.Infow("migrations completed")
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	kafka := messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafka.Close()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		appLog.Fatalw("listen failed", zap.Error(err))
	}

	poolCfg, err := pgxpool.ParseConfig(pgxURL(env.GetString("DB_URL", "")))
	if err != nil {
		appLog.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnIdleTime = 15 * time.Minute

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		appLog.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	rdb := newRedisClient(
		env.GetString("REDIS_URL", "redis:6379"),
		env.GetString("REDIS_PASSWORD", ""),
	)

	pgStore := store.NewPostgresStore(db)
	svc := NewService(pgStore, rdb, appLog)

	consumer := NewTripConsumer(kafka, svc)
	consumer.Start(ctx)

	grpcServer := grpcserver.NewServer(append(tracing.WithTracingInterceptors(),
		grpcserver.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
	)...)
	NewGrpcHandler(grpcServer, svc, consumer)

	appLog.Infow("driver-service ready", "addr", grpcAddr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			appLog.Errorw("grpc serve failed", zap.Error(err))
			cancel()
		}
	}()

	<-ctx.Done()
	appLog.Infow("driver-service shutting down")
	grpcServer.GracefulStop()
}

func newRedisClient(addr, password string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		appLog.Fatalw("redis connect failed", "err", err)
	}
	return rdb
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
