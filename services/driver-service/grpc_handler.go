package main

import (
	"context"
	"io"
	"log"

	pb "drova/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type driverGrpcHandler struct {
	pb.UnimplementedDriverServiceServer
	service *Service
}

func NewGrpcHandler(s *grpc.Server, service *Service) {
	handler := &driverGrpcHandler{service: service}
	pb.RegisterDriverServiceServer(s, handler)
}

func (h *driverGrpcHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	driver, err := h.service.RegisterDriver(req.GetDriverID(), req.GetPackageSlug())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register driver")
	}

	return &pb.RegisterDriverResponse{Driver: driver}, nil
}

func (h *driverGrpcHandler) UnregisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	h.service.UnregisterDriver(req.GetDriverID())

	return &pb.RegisterDriverResponse{
		Driver: &pb.Driver{Id: req.GetDriverID()},
	}, nil
}

func (h *driverGrpcHandler) StreamLocation(stream grpc.ClientStreamingServer[pb.LocationUpdate, pb.StreamLocationResponse]) error {
	var driverID string
	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.StreamLocationResponse{})
		}
		if err != nil {
			if driverID != "" {
				log.Printf("stream broken for driver %s — unregistering", driverID)
				h.service.UnregisterDriver(driverID)
			}
			return err
		}
		driverID = update.GetDriverId()
		h.service.UpdateLocation(update.GetDriverId(), update.GetLatitude(), update.GetLongitude())
	}
}
