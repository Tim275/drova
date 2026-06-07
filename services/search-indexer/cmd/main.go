package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"drova/services/search-indexer/internal/indexer"
	"drova/shared/env"
	"drova/shared/logger"
	"drova/shared/messaging"
)

func main() {
	envStr := env.GetString("ENVIRONMENT", "development")
	log := logger.New(envStr, "search-indexer")
	defer log.Sync()

	log.Infow("search-indexer starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	es := indexer.NewES(env.GetString("ELASTICSEARCH_URL", "http://elasticsearch:9200"))
	for {
		if err := es.EnsureIndex(ctx); err != nil {
			log.Warnw("waiting for elasticsearch", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}
		break
	}
	log.Infow("elasticsearch index ready")

	kafkaClient := messaging.NewKafka(strings.Split(env.GetString("KAFKA_BROKERS", "localhost:9092"), ","))
	defer kafkaClient.Close()

	idx := indexer.New(kafkaClient, es, log)
	idx.Start(ctx)

	healthAddr := env.GetString("HEALTH_ADDR", ":8086")
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := es.EnsureIndex(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
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

	log.Infow("search-indexer ready", "health", healthAddr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Infow("search-indexer shutting down")
	cancel()
	kafkaClient.Wait()
}
