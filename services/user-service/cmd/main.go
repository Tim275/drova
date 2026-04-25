package main

import (
	"context"
	"expvar"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/mailer"
	"drova/services/user-service/internal/middleware"
	"drova/services/user-service/internal/service"
	"drova/services/user-service/internal/store"
	"drova/shared/env"
	"drova/shared/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type application struct {
	service *service.UserService
	auth    *auth.Authenticator
	log     *zap.SugaredLogger
	rdb     *redis.Client
}

func main() {
	log := logger.New(env.GetString("ENVIRONMENT", "development"))
	defer log.Sync()

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
		Addr: env.GetString("REDIS_URL", "localhost:6379"),
	})
	defer rdb.Close()

	migrationsPath := env.GetString("MIGRATIONS_PATH", "services/user-service/migrations")
	if err := runMigrations(env.GetString("DB_URL", ""), migrationsPath); err != nil {
		log.Fatalw("migrations", zap.Error(err))
	}

	m := mailer.New(
		env.GetString("MAILTRAP_HOST", "sandbox.smtp.mailtrap.io"),
		env.GetString("MAILTRAP_PORT", "2525"),
		env.GetString("MAILTRAP_USERNAME", ""),
		env.GetString("MAILTRAP_PASSWORD", ""),
	)

	authenticator := auth.NewAuthenticator(
		env.GetString("JWT_SECRET", ""),
		"drova",
		"drova-users",
	)

	userStore := store.NewUserStore(db)
	invStore := store.NewInvitationStore(db)
	userCache := store.NewUserCache(rdb)
	svc := service.New(userStore, invStore, userCache, m)

	app := &application{service: svc, auth: authenticator, log: log, rdb: rdb}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", app.handleHealth)
	mux.HandleFunc("POST /v1/users/register", app.handleRegister)
	mux.HandleFunc("GET /v1/users/activate/{token}", app.handleActivate)
	mux.HandleFunc("POST /v1/auth/token", app.handleCreateToken)
	mux.Handle("GET /debug/vars", expvar.Handler())

	protected := middleware.Authenticate(authenticator)
	mux.Handle("GET /v1/users/me", protected(http.HandlerFunc(app.handleGetMe)))

	handler := middleware.Metrics(middleware.RateLimit(mux))

	addr := env.GetString("HTTP_ADDR", ":8082")
	srv := &http.Server{Addr: addr, Handler: handler}

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
