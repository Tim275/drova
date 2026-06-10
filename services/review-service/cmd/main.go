package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
	"drova/shared/tracing"

	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
)

var (
	httpAddr     = env.GetString("HTTP_ADDR", ":8085")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:           "review-service",
		Environment:           envStr,
		OtelCollectorEndpoint: env.GetString("OTEL_COLLECTOR_ENDPOINT", ""),
	})
	defer stopTracer(context.Background())

	log := logger.New(envStr, "review-service")
	defer log.Sync()

	if err != nil {
		log.Warnw("tracing init failed", zap.Error(err))
	}

	log.Infow("review-service starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kafka := messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafka.Close()

	_ = kafka

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Infow("http server ready", "addr", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorw("http server error", zap.Error(err))
			cancel()
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case <-shutdown:
		log.Infow("review-service shutting down")
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorw("http shutdown", zap.Error(err))
	}

	kafka.Wait()
}
