package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"drova/shared/env"
	"drova/shared/messaging"
	"drova/shared/tracing"

	grpcserver "google.golang.org/grpc"
)

var (
	grpcAddr     = env.GetString("GRPC_ADDR", ":9092")
	kafkaBrokers = env.GetString("KAFKA_BROKERS", "kafka:9092")
)

func main() {
	log.Println("Starting Driver Service")

	stopTracer, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "driver-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "jaeger:4318"),
	})
	if err != nil {
		log.Printf("warning: tracing init failed: %v", err)
	} else {
		defer stopTracer(context.Background())
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
		log.Fatalf("failed to listen: %v", err)
	}

	svc := NewService()

	consumer := NewTripConsumer(kafka, svc)
	consumer.Start(ctx)

	grpcServer := grpcserver.NewServer(tracing.WithTracingInterceptors()...)
	NewGrpcHandler(grpcServer, svc)

	log.Printf("gRPC server listening on %s", lis.Addr().String())

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("failed to serve: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down driver service")
	grpcServer.GracefulStop()
}
