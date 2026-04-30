package grpc_clients

import (
	"fmt"
	"os"
	"sync"
	"time"

	pbd "drova/shared/proto/driver"
	pbt "drova/shared/proto/trip"
	pbu "drova/shared/proto/user"
	"drova/shared/tracing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	tripConn   *grpc.ClientConn
	driverConn *grpc.ClientConn
	userConn   *grpc.ClientConn
	once       sync.Once

	TripClient   pbt.TripServiceClient
	DriverClient pbd.DriverServiceClient
	UserClient   pbu.UserServiceClient
)

func InitSharedClients() error {
	var initErr error
	once.Do(func() {
		opts := append(
			tracing.DialOptionsWithTracing(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                5 * time.Minute,
				Timeout:             20 * time.Second,
				PermitWithoutStream: false,
			}),
		)

		tripURL := os.Getenv("TRIP_SERVICE_URL")
		if tripURL == "" {
			tripURL = "trip-service:9093"
		}
		tc, err := grpc.NewClient(tripURL, opts...)
		if err != nil {
			initErr = fmt.Errorf("trip service dial: %w", err)
			return
		}
		tripConn = tc
		TripClient = pbt.NewTripServiceClient(tc)

		driverURL := os.Getenv("DRIVER_SERVICE_URL")
		if driverURL == "" {
			driverURL = "driver-service:9092"
		}
		dc, err := grpc.NewClient(driverURL, opts...)
		if err != nil {
			initErr = fmt.Errorf("driver service dial: %w", err)
			return
		}
		driverConn = dc
		DriverClient = pbd.NewDriverServiceClient(dc)

		userURL := os.Getenv("USER_SERVICE_GRPC_URL")
		if userURL == "" {
			userURL = "user-service:9091"
		}
		uc, err := grpc.NewClient(userURL, opts...)
		if err != nil {
			initErr = fmt.Errorf("user service dial: %w", err)
			return
		}
		userConn = uc
		UserClient = pbu.NewUserServiceClient(uc)
	})
	return initErr
}

func CloseSharedClients() {
	if tripConn != nil {
		tripConn.Close()
	}
	if driverConn != nil {
		driverConn.Close()
	}
	if userConn != nil {
		userConn.Close()
	}
}
