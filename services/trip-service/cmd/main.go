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

	"drova/services/trip-service/internal/domain"
	"drova/services/trip-service/internal/infrastructure/events"
	grpcHandler "drova/services/trip-service/internal/infrastructure/grpc"
	"drova/services/trip-service/internal/infrastructure/grpc/userinfo"
	"drova/services/trip-service/internal/infrastructure/repository"
	"drova/services/trip-service/internal/service"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	pbu "drova/shared/proto/user"
	"drova/shared/tracing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	grpcserver "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	grpcAddr     = env.GetString("GRPC_ADDR", ":9093")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "trip-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	log := logger.New(envStr, "trip-service")
	defer log.Sync()

	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
	}

	log.Infow("trip-service starting")

	if err := runMigrations(env.GetString("DB_URL", ""), env.GetString("MIGRATIONS_PATH", "services/trip-service/migrations")); err != nil {
		log.Fatalw("migrations failed", zap.Error(err))
	}
	log.Infow("migrations completed")
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kafka := messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafka.Close()

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

	userGRPCURL := env.GetString("USER_SERVICE_GRPC_URL", "user-service:9091")
	userConn, err := grpcserver.NewClient(userGRPCURL,
		grpcserver.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Warnw("user service grpc dial failed, rider info will be empty", "err", err)
	}
	var userInfoProvider domain.UserInfoProvider
	if userConn != nil {
		defer userConn.Close()
		userInfoProvider = userinfo.New(pbu.NewUserServiceClient(userConn))
	} else {
		userInfoProvider = &noopUserInfo{}
	}

	publisher := events.NewTripEventPublisher(kafka, log)

	pgRepo := repository.NewPostgresRepository(db)
	svc := service.NewService(pgRepo, userInfoProvider, nil)

	driverConsumer := events.NewDriverConsumer(kafka, svc, log)
	paymentConsumer := events.NewPaymentConsumer(kafka, svc, log)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalw("listen failed", zap.Error(err))
	}

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
	grpcHandler.NewGRPCHandler(grpcSrv, svc, publisher, log)

	driverConsumer.Start(ctx)
	paymentConsumer.Start(ctx)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := svc.ExpireStaleSearching(ctx, 5*time.Minute)
				if err != nil {
					log.Warnw("expire stale searching trips", "err", err)
				} else if n > 0 {
					log.Infow("expired stale searching trips", "count", n)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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

	kafka.Wait()
	grpcSrv.GracefulStop()
}

// noopUserInfo is used when user-service gRPC is unavailable at startup
type noopUserInfo struct{}

func (n *noopUserInfo) GetRiderInfo(_ context.Context, _ string) (string, string) { return "", "" }

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
