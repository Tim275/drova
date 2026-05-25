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

	"drova/services/driver-service/internal/events"
	drivergrpc "drova/services/driver-service/internal/grpc"
	"drova/services/driver-service/internal/service"
	"drova/services/driver-service/internal/store"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
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

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "driver-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	log := logger.New(envStr, "driver-service")
	defer log.Sync()

	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
	}

	log.Infow("driver-service starting")

	if err := runMigrations(env.GetString("DB_URL", ""), env.GetString("MIGRATIONS_PATH", "services/driver-service/migrations")); err != nil {
		log.Fatalw("migrations failed", zap.Error(err))
	}
	log.Infow("migrations completed")
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
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
		log.Fatalw("listen failed", zap.Error(err))
	}

	poolURL := env.GetString("POOL_URL", env.GetString("DB_URL", ""))
	poolCfg, err := pgxpool.ParseConfig(pgxURL(poolURL))
	if err != nil {
		log.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 2
	poolCfg.MaxConnIdleTime = 15 * time.Minute
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	rdb := newRedisClient(env.GetString("REDIS_URL", "redis:6379"), env.GetString("REDIS_PASSWORD", ""), log)

	pgStore := store.NewPostgresStore(db)
	svc := service.NewService(pgStore, rdb, log)

	consumer := events.NewTripConsumer(kafka, svc, rdb, log)
	svc.OnDriverOnline = consumer.TryMatchWaiting
	consumer.Start(ctx)

	grpcSrv := grpcserver.NewServer(append(tracing.WithTracingInterceptors(),
		grpcserver.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 30 * time.Second,
			Time:              20 * time.Second,
			Timeout:           5 * time.Second,
		}),
		grpcserver.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)...)
	drivergrpc.NewHandler(grpcSrv, svc, consumer, log)

	log.Infow("driver-service ready", "addr", grpcAddr)

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			log.Errorw("grpc serve failed", zap.Error(err))
			cancel()
		}
	}()

	<-ctx.Done()
	log.Infow("driver-service shutting down")
	kafka.Wait()
	grpcSrv.GracefulStop()
}

func newRedisClient(addr, password string, log *zap.SugaredLogger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalw("redis connect failed", "err", err)
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

func runMigrations(dbURL, migrationsPath string) error {
	stripped := strings.TrimPrefix(dbURL, "postgresql://")
	stripped = strings.TrimPrefix(stripped, "postgres://")
	migrateURL := "pgx5://" + stripped
	m, err := migrate.New("file://"+migrationsPath, migrateURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
