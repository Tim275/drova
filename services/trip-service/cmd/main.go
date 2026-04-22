package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpcHandler "drova/services/trip-service/internal/infrastructure/grpc"
	"drova/services/trip-service/internal/infrastructure/repository"
	"drova/services/trip-service/internal/service"
	"drova/shared/env"

	grpcserver "google.golang.org/grpc"
)

var grpcAddr = env.GetString("GRPC_ADDR", ":9093")

func main() {
	log.Println("Starting Trip Service")

	inmemRepo := repository.NewInmemRepository()
	svc := service.NewService(inmemRepo)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcAddr, err)
	}

	grpcSrv := grpcserver.NewServer()
	grpcHandler.NewGRPCHandler(grpcSrv, svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
