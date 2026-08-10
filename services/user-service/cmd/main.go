package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
	grpc_handler "drova/services/user-service/internal/grpc"
	"drova/services/user-service/internal/mailer"
	"drova/services/user-service/internal/middleware"
	"drova/services/user-service/internal/service"
	"drova/services/user-service/internal/store"
	"drova/shared/env"
	"drova/shared/logger"
	pb "drova/shared/proto/user"
	"drova/shared/tracing"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

type application struct {
	service       *service.UserService
	auth          *auth.Authenticator
	blacklist     domain.TokenBlacklist
	refreshTokens domain.RefreshTokenStore
	log           *zap.SugaredLogger
	db            *pgxpool.Pool
	rdb           *redis.Client
}

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "user-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	log := logger.New(envStr, "user-service")
	defer log.Sync()
	tracing.StartPprofServer(log, ":6060")

	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
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

	poolURL := env.GetString("POOL_URL", env.GetString("DB_URL", ""))
	poolCfg, err := pgxpool.ParseConfig(poolURL)
	if err != nil {
		log.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 25
	poolCfg.MinConns = 5
	poolCfg.MaxConnIdleTime = 15 * time.Minute
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     env.GetString("REDIS_URL", "localhost:6379"),
		Password: env.GetString("REDIS_PASSWORD", ""),
	})
	if err := redisotel.InstrumentTracing(rdb, redisotel.WithDBStatement(false)); err != nil {
		log.Warnw("redis tracing instrumentation failed", zap.Error(err))
	}
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalw("redis ping failed", zap.Error(err))
	}

	migrationsPath := env.GetString("MIGRATIONS_PATH", "services/user-service/migrations")
	if err := runMigrations(env.GetString("DB_URL", ""), migrationsPath); err != nil {
		log.Fatalw("migrations", zap.Error(err))
	}

	if env.GetString("SEED", "") == "true" {
		store.Seed(context.Background(), db, log)
	}

	m := mailer.New(
		env.GetString("SMTP_HOST", "smtp.gmail.com"),
		env.GetString("SMTP_PORT", "587"),
		env.GetString("SMTP_USERNAME", ""),
		env.GetString("SMTP_PASSWORD", ""),
		env.GetString("FROM_EMAIL", ""),
		env.GetString("APP_BASE_URL", "http://localhost:8081"),
	)

	jwtSecret := env.GetString("JWT_SECRET", "")
	if len(jwtSecret) < 32 {
		log.Fatalw("JWT_SECRET must be at least 32 characters")
	}
	authenticator := auth.NewAuthenticator(jwtSecret, "drova", "drova-users")

	userStore := store.NewUserStore(db)
	invStore := store.NewInvitationStore(db)
	userCache := store.NewUserCache(rdb)
	blacklist := store.NewTokenBlacklist(rdb)
	refreshTokens := store.NewRefreshTokenStore(rdb)
	svc := service.New(userStore, invStore, userCache, m, log)

	app := &application{service: svc, auth: authenticator, blacklist: blacklist, refreshTokens: refreshTokens, log: log, db: db, rdb: rdb}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", app.handleHealth)
	mux.HandleFunc("GET /v1/readyz", app.handleReadyz)
	mux.Handle("POST /v1/users/register", tracing.WrapHandlerFunc(app.handleRegister, "POST /v1/users/register"))
	mux.Handle("GET /v1/users/activate/{token}", tracing.WrapHandlerFunc(app.handleActivate, "GET /v1/users/activate"))
	mux.Handle("POST /v1/auth/token", tracing.WrapHandlerFunc(app.handleCreateToken, "POST /v1/auth/token"))
	protected := middleware.Authenticate(authenticator, blacklist)
	mux.Handle("GET /v1/users/me", tracing.WrapHandler(protected(http.HandlerFunc(app.handleGetMe)), "GET /v1/users/me"))
	mux.Handle("PATCH /v1/users/profile", tracing.WrapHandler(protected(http.HandlerFunc(app.handleUpdateProfile)), "PATCH /v1/users/profile"))
	mux.Handle("POST /v1/auth/logout", tracing.WrapHandler(protected(http.HandlerFunc(app.handleLogout)), "POST /v1/auth/logout"))
	mux.Handle("POST /v1/auth/refresh", tracing.WrapHandlerFunc(app.handleRefreshToken, "POST /v1/auth/refresh"))

	handler := middleware.Metrics(middleware.RateLimit(rdb)(mux))

	addr := env.GetString("HTTP_ADDR", ":8082")
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// gRPC server
	grpcAddr := env.GetString("GRPC_ADDR", ":9091")
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalw("grpc listen", zap.Error(err))
	}
	grpcSrv := grpc.NewServer(append(tracing.WithTracingInterceptors(),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 30 * time.Second,
			Time:              20 * time.Second,
			Timeout:           5 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)...)
	pb.RegisterUserServiceServer(grpcSrv, grpc_handler.New(svc, authenticator, blacklist, log))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		log.Infow("user-service gRPC started", "addr", grpcAddr)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Errorw("grpc serve", zap.Error(err))
		}
	}()

	go func() {
		log.Infow("user-service HTTP started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorw("shutdown", zap.Error(err))
	}
}
