package main

import (
	"context"
	"io"
	"time"

	pb "drova/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type driverGrpcHandler struct {
	pb.UnimplementedDriverServiceServer
	service  *Service
	consumer *TripConsumer
}

func NewGrpcHandler(s *grpc.Server, service *Service, consumer *TripConsumer) {
	handler := &driverGrpcHandler{service: service, consumer: consumer}
	pb.RegisterDriverServiceServer(s, handler)
}

func (h *driverGrpcHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	driver, err := h.service.RegisterDriver(ctx, req.GetDriverID(), req.GetPackageSlug(), req.GetName(), req.GetLat(), req.GetLng())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register driver")
	}

	busyTripID := h.service.GetBusyTripID(ctx, req.GetDriverID())
	if busyTripID != "" {
		if !h.consumer.ExtendTimerForDriver(ctx, busyTripID, req.GetDriverID()) {
			// No pending timer — service restarted or trip already resolved.
			// Clear stale busy state so the driver is immediately available.
			h.service.ClearBusy(ctx, req.GetDriverID())
			busyTripID = ""
		}
	}
	if busyTripID == "" {
		loc := driver.GetLocation()
		if loc != nil {
			slug := driver.GetPackageSlug()
			lat := loc.GetLatitude()
			lng := loc.GetLongitude()
			// Run after a short delay: the api-gateway sets up the driver's Redis
			// pub/sub subscription only AFTER this gRPC call returns. If we call
			// TryMatchWaiting synchronously, the Kafka notification arrives before
			// the subscription is active and the driver never sees the trip request.
			go func() {
				time.Sleep(400 * time.Millisecond)
				h.consumer.TryMatchWaiting(context.Background(), slug, lat, lng)
			}()
		}
	}

	return &pb.RegisterDriverResponse{
		Driver:         driver,
		BusyWithTripId: busyTripID,
	}, nil
}

func (h *driverGrpcHandler) UnregisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	h.service.UnregisterDriver(ctx, req.GetDriverID())
	return &pb.RegisterDriverResponse{
		Driver: &pb.Driver{Id: req.GetDriverID()},
	}, nil
}

func (h *driverGrpcHandler) StreamLocation(stream grpc.ClientStreamingServer[pb.LocationUpdate, pb.StreamLocationResponse]) error {
	ctx := stream.Context()
	var driverID string
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.StreamLocationResponse{})
		}
		if err != nil {
			if driverID != "" {
				h.service.log.Infow("location stream broken, unregistering driver", "driver", driverID)
				h.service.UnregisterDriver(ctx, driverID)
			}
			return err
		}
		driverID = update.GetDriverId()
		h.service.UpdateLocation(ctx, update.GetDriverId(), update.GetLatitude(), update.GetLongitude())
	}
}
