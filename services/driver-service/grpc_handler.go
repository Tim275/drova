package main

import (
	"context"
	"io"

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
	driver, err := h.service.RegisterDriver(ctx, req.GetDriverID(), req.GetPackageSlug(), req.GetName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register driver")
	}

	loc := driver.GetLocation()
	if loc != nil {
		h.consumer.TryMatchWaiting(ctx, driver.GetPackageSlug(), loc.GetLatitude(), loc.GetLongitude())
	}

	return &pb.RegisterDriverResponse{Driver: driver}, nil
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
