package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/mailer"
	"drova/services/user-service/internal/middleware"
	"drova/services/user-service/internal/service"
	"drova/services/user-service/internal/store"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/tracing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type application struct {
	service   *service.UserService
	auth      *auth.Authenticator
	blacklist domain.TokenBlacklist
	log       *zap.SugaredLogger
	rdb       *redis.Client
}

func main() {
	log := logger.New(env.GetString("ENVIRONMENT", "development"))
	defer log.Sync()

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "user-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
	} else {
		defer stopTracer(context.Background())
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := runMigrations(
			env.GetString("DB_URL", ""),
			env.GetString("MIGRATIONS_PATH", "migrations"),
		); err != nil {
			log.Fatalw("migrations failed", zap.Error(err))
		}
		log.Infow("migrations complete")
		return
	}

	poolCfg, err := pgxpool.ParseConfig(env.GetString("DB_URL", ""))
	if err != nil {
		log.Fatalw("db config", zap.Error(err))
	}
	poolCfg.MaxConns = 25
	poolCfg.MinConns = 5
	poolCfg.MaxConnIdleTime = 15 * time.Minute

	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Fatalw("db connect", zap.Error(err))
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     env.GetString("REDIS_URL", "localhost:6379"),
		Password: env.GetString("REDIS_PASSWORD", ""),
	})
	defer rdb.Close()

	migrationsPath := env.GetString("MIGRATIONS_PATH", "services/user-service/migrations")
	if err := runMigrations(env.GetString("DB_URL", ""), migrationsPath); err != nil {
		log.Fatalw("migrations", zap.Error(err))
	}

	if env.GetString("SEED", "") == "true" {
		store.Seed(context.Background(), db)
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
	svc := service.New(userStore, invStore, userCache, m)

	app := &application{service: svc, auth: authenticator, blacklist: blacklist, log: log, rdb: rdb}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", app.handleHealth)
	mux.Handle("POST /v1/users/register", tracing.WrapHandlerFunc(app.handleRegister, "POST /v1/users/register"))
	mux.Handle("GET /v1/users/activate/{token}", tracing.WrapHandlerFunc(app.handleActivate, "GET /v1/users/activate"))
	mux.Handle("POST /v1/auth/token", tracing.WrapHandlerFunc(app.handleCreateToken, "POST /v1/auth/token"))
	mux.Handle("GET /v1/internal/users/{id}", tracing.WrapHandlerFunc(app.handleInternalGetUser, "GET /v1/internal/users"))

	protected := middleware.Authenticate(authenticator, blacklist)
	mux.Handle("GET /v1/users/me", tracing.WrapHandler(protected(http.HandlerFunc(app.handleGetMe)), "GET /v1/users/me"))
	mux.Handle("PATCH /v1/users/profile", tracing.WrapHandler(protected(http.HandlerFunc(app.handleUpdateProfile)), "PATCH /v1/users/profile"))
	mux.Handle("POST /v1/auth/logout", tracing.WrapHandler(protected(http.HandlerFunc(app.handleLogout)), "POST /v1/auth/logout"))

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

	go func() {
		log.Infow("user-service started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorw("shutdown", zap.Error(err))
	}
}
