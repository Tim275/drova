
package trip

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const (
	TripService_PreviewTrip_FullMethodName = "/trip.TripService/PreviewTrip"
	TripService_CreateTrip_FullMethodName  = "/trip.TripService/CreateTrip"
)

type TripServiceClient interface {
	PreviewTrip(ctx context.Context, in *PreviewTripRequest, opts ...grpc.CallOption) (*PreviewTripResponse, error)
	CreateTrip(ctx context.Context, in *CreateTripRequest, opts ...grpc.CallOption) (*CreateTripResponse, error)
}

type tripServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTripServiceClient(cc grpc.ClientConnInterface) TripServiceClient {
	return &tripServiceClient{cc}
}

func (c *tripServiceClient) PreviewTrip(ctx context.Context, in *PreviewTripRequest, opts ...grpc.CallOption) (*PreviewTripResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PreviewTripResponse)
	err := c.cc.Invoke(ctx, TripService_PreviewTrip_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tripServiceClient) CreateTrip(ctx context.Context, in *CreateTripRequest, opts ...grpc.CallOption) (*CreateTripResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateTripResponse)
	err := c.cc.Invoke(ctx, TripService_CreateTrip_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type TripServiceServer interface {
	PreviewTrip(context.Context, *PreviewTripRequest) (*PreviewTripResponse, error)
	CreateTrip(context.Context, *CreateTripRequest) (*CreateTripResponse, error)
	mustEmbedUnimplementedTripServiceServer()
}

type UnimplementedTripServiceServer struct{}

func (UnimplementedTripServiceServer) PreviewTrip(context.Context, *PreviewTripRequest) (*PreviewTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PreviewTrip not implemented")
}
func (UnimplementedTripServiceServer) CreateTrip(context.Context, *CreateTripRequest) (*CreateTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTrip not implemented")
}
func (UnimplementedTripServiceServer) mustEmbedUnimplementedTripServiceServer() {}
func (UnimplementedTripServiceServer) testEmbeddedByValue()                     {}

type UnsafeTripServiceServer interface {
	mustEmbedUnimplementedTripServiceServer()
}

func RegisterTripServiceServer(s grpc.ServiceRegistrar, srv TripServiceServer) {
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&TripService_ServiceDesc, srv)
}

func _TripService_PreviewTrip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PreviewTripRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).PreviewTrip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_PreviewTrip_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).PreviewTrip(ctx, req.(*PreviewTripRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TripService_CreateTrip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateTripRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).CreateTrip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_CreateTrip_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).CreateTrip(ctx, req.(*CreateTripRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var TripService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "trip.TripService",
	HandlerType: (*TripServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PreviewTrip",
			Handler:    _TripService_PreviewTrip_Handler,
		},
		{
			MethodName: "CreateTrip",
			Handler:    _TripService_CreateTrip_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/trip.proto",
}
