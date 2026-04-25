package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	grpcHandler "drova/services/trip-service/internal/infrastructure/grpc"
	"drova/services/trip-service/internal/infrastructure/events"
	"drova/services/trip-service/internal/infrastructure/repository"
	"drova/services/trip-service/internal/service"
	"drova/shared/env"
	"drova/shared/messaging"
	"drova/shared/tracing"

	grpcserver "google.golang.org/grpc"
)

var (
	grpcAddr     = env.GetString("GRPC_ADDR", ":9093")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

func main() {
	log.Println("Starting Trip Service")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "trip-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Printf("warning: tracing init failed: %v", err)
	} else {
		defer stopTracer(context.Background())
	}

	kafka := messaging.NewKafka(strings.Split(kafkaBrokers, ","))
	defer kafka.Close()

	publisher := events.NewTripEventPublisher(kafka)

	inmemRepo := repository.NewInmemRepository()
	svc := service.NewService(inmemRepo)

	driverConsumer := events.NewDriverConsumer(kafka, svc)
	paymentConsumer := events.NewPaymentConsumer(kafka, svc)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcAddr, err)
	}

	grpcSrv := grpcserver.NewServer(tracing.WithTracingInterceptors()...)
	grpcHandler.NewGRPCHandler(grpcSrv, svc, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driverConsumer.Start(ctx)
	paymentConsumer.Start(ctx)

	go func() {
		log.Printf("gRPC server listening on %s", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
			cancel()
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case <-shutdown:
		log.Println("Shutting down trip service")
	case <-ctx.Done():
	}

	grpcSrv.GracefulStop()
}
