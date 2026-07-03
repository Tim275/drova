package grpc

import (
	"context"
	"strings"
	"time"

	"drova/services/trip-service/internal/domain"
	"drova/services/trip-service/internal/infrastructure/events"
	"drova/shared/contracts"
	pb "drova/shared/proto/trip"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const tripSearchTimeout = 2 * time.Minute

type gRPCHandler struct {
	pb.UnimplementedTripServiceServer
	service domain.TripService
	log     *zap.SugaredLogger
}

func NewGRPCHandler(server *grpc.Server, service domain.TripService, log *zap.SugaredLogger) *gRPCHandler {
	handler := &gRPCHandler{service: service, log: log}
	pb.RegisterTripServiceServer(server, handler)
	return handler
}

func (h *gRPCHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	fare, err := h.service.GetAndValidateFare(ctx, req.GetRideFareID(), req.GetUserID())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid fare: %v", err)
	}

	trip, err := h.service.CreateTrip(ctx, fare)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create trip: %v", err)
	}

	go h.runSearchTimeout(trip.ID, trip.UserID) //nolint:gosec

	return &pb.CreateTripResponse{
		TripID: trip.ID,
	}, nil
}

func (h *gRPCHandler) runSearchTimeout(tripID, riderID string) {
	timer := time.NewTimer(tripSearchTimeout)
	defer timer.Stop()
	<-timer.C

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msgs, err := events.BuildSearchTimeout(tripID, riderID)
	if err != nil {
		h.log.Warnw("build search timeout", "trip", tripID, zap.Error(err))
		return
	}
	cancelled, err := h.service.ExpireSearchWithOutbox(ctx, tripID, msgs)
	if err != nil {
		h.log.Warnw("expire search failed", "trip", tripID, zap.Error(err))
		return
	}
	if !cancelled {
		return // trip already accepted, completed, or manually cancelled
	}
	h.log.Infow("trip search timed out", "trip", tripID, "rider", riderID)
}

func (h *gRPCHandler) GetTripsByUser(ctx context.Context, req *pb.GetTripsRequest) (*pb.TripsResponse, error) {
	trips, err := h.service.GetTripsByUser(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get trips: %v", err)
	}
	items := make([]*pb.TripHistoryItem, 0, len(trips))
	for _, t := range trips {
		item := &pb.TripHistoryItem{
			Id:              t.ID,
			Status:          t.Status,
			Rating:          int32(t.Rating), // #nosec G115 -- rating is 1-5, safely fits int32
			CreatedAt:       t.CreatedAt,
			DriverName:      t.DriverName,
			DriverPlate:     t.DriverPlate,
			DriverAvatar:    t.DriverAvatar,
			PickupAddress:   t.PickupAddress,
			DropoffAddress:  t.DropoffAddress,
			DistanceMeters:  t.DistanceMeters,
			DurationSeconds: t.DurationSeconds,
			PackageSlug:     t.PackageSlug,
			PriceCents:      t.AmountCents,
		}
		if t.Fare != nil && item.PriceCents == 0 {
			item.PackageSlug = t.Fare.PackageSlug
			item.PriceCents = t.Fare.TotalPriceInCents
		}
		items = append(items, item)
	}
	return &pb.TripsResponse{Trips: items}, nil
}

func (h *gRPCHandler) GetTripsByDriver(ctx context.Context, req *pb.GetTripsRequest) (*pb.DriverTripsResponse, error) {
	trips, err := h.service.GetTripsByDriver(ctx, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get driver trips: %v", err)
	}
	items := make([]*pb.DriverTripHistoryItem, 0, len(trips))
	for _, t := range trips {
		item := &pb.DriverTripHistoryItem{
			Id:              t.ID,
			Status:          t.Status,
			RiderName:       t.RiderName,
			RiderAvatar:     t.RiderAvatar,
			CreatedAt:       t.CreatedAt,
			PickupAddress:   t.PickupAddress,
			DropoffAddress:  t.DropoffAddress,
			DistanceMeters:  t.DistanceMeters,
			DurationSeconds: t.DurationSeconds,
			PackageSlug:     t.PackageSlug,
			PriceCents:      t.AmountCents,
		}
		if t.Fare != nil && item.PriceCents == 0 {
			item.PackageSlug = t.Fare.PackageSlug
			item.PriceCents = t.Fare.TotalPriceInCents
		}
		items = append(items, item)
	}
	return &pb.DriverTripsResponse{Trips: items}, nil
}

func (h *gRPCHandler) RateTrip(ctx context.Context, req *pb.RateTripRequest) (*pb.RateTripResponse, error) {
	userID := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("user-id"); len(v) > 0 {
			userID = v[0]
		}
	}
	if err := h.service.RateTrip(ctx, req.GetTripID(), userID, int(req.GetRating())); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "rate trip: %v", err)
	}
	return &pb.RateTripResponse{}, nil
}

func (h *gRPCHandler) CancelTrip(ctx context.Context, req *pb.CancelTripRequest) (*pb.CancelTripResponse, error) {
	if err := h.service.CancelTrip(ctx, req.GetTripID()); err != nil {
		return nil, status.Errorf(codes.Internal, "cancel trip: %v", err)
	}
	return &pb.CancelTripResponse{}, nil
}

func (h *gRPCHandler) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {
	pickup := &contracts.Coordinate{
		Latitude:  req.GetStartLocation().Latitude,
		Longitude: req.GetStartLocation().Longitude,
	}
	destination := &contracts.Coordinate{
		Latitude:  req.GetEndLocation().Latitude,
		Longitude: req.GetEndLocation().Longitude,
	}

	route, err := h.service.GetRoute(ctx, pickup, destination)
	if err != nil {
		h.log.Warnw("get route failed", zap.Error(err))
		if strings.Contains(err.Error(), "no route") {
			return nil, status.Errorf(codes.InvalidArgument, "no route found between the given locations")
		}
		return nil, status.Errorf(codes.Internal, "failed to get route: %v", err)
	}

	estimatedFares := h.service.EstimatePackagesPriceWithRoute(route)

	// Attach addresses + coordinates to the first fare so GenerateTripFares propagates them to all
	if len(estimatedFares) > 0 {
		estimatedFares[0].PickupAddress = req.GetPickupAddress()
		estimatedFares[0].DropoffAddress = req.GetDropoffAddress()
		estimatedFares[0].PickupLat = req.GetStartLocation().GetLatitude()
		estimatedFares[0].PickupLng = req.GetStartLocation().GetLongitude()
		estimatedFares[0].DropoffLat = req.GetEndLocation().GetLatitude()
		estimatedFares[0].DropoffLng = req.GetEndLocation().GetLongitude()
	}

	fares, err := h.service.GenerateTripFares(ctx, estimatedFares, req.GetUserID(), route)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate ride fares: %v", err)
	}

	return &pb.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: domain.ToRideFaresProto(fares),
	}, nil
}
