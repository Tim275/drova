
package trip

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const (
	TripService_PreviewTrip_FullMethodName      = "/trip.TripService/PreviewTrip"
	TripService_CreateTrip_FullMethodName       = "/trip.TripService/CreateTrip"
	TripService_GetTripsByUser_FullMethodName   = "/trip.TripService/GetTripsByUser"
	TripService_GetTripsByDriver_FullMethodName = "/trip.TripService/GetTripsByDriver"
	TripService_RateTrip_FullMethodName         = "/trip.TripService/RateTrip"
	TripService_CancelTrip_FullMethodName       = "/trip.TripService/CancelTrip"
)

type TripServiceClient interface {
	PreviewTrip(ctx context.Context, in *PreviewTripRequest, opts ...grpc.CallOption) (*PreviewTripResponse, error)
	CreateTrip(ctx context.Context, in *CreateTripRequest, opts ...grpc.CallOption) (*CreateTripResponse, error)
	GetTripsByUser(ctx context.Context, in *GetTripsRequest, opts ...grpc.CallOption) (*TripsResponse, error)
	GetTripsByDriver(ctx context.Context, in *GetTripsRequest, opts ...grpc.CallOption) (*DriverTripsResponse, error)
	RateTrip(ctx context.Context, in *RateTripRequest, opts ...grpc.CallOption) (*RateTripResponse, error)
	CancelTrip(ctx context.Context, in *CancelTripRequest, opts ...grpc.CallOption) (*CancelTripResponse, error)
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

func (c *tripServiceClient) GetTripsByUser(ctx context.Context, in *GetTripsRequest, opts ...grpc.CallOption) (*TripsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TripsResponse)
	err := c.cc.Invoke(ctx, TripService_GetTripsByUser_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tripServiceClient) GetTripsByDriver(ctx context.Context, in *GetTripsRequest, opts ...grpc.CallOption) (*DriverTripsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DriverTripsResponse)
	err := c.cc.Invoke(ctx, TripService_GetTripsByDriver_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tripServiceClient) RateTrip(ctx context.Context, in *RateTripRequest, opts ...grpc.CallOption) (*RateTripResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RateTripResponse)
	err := c.cc.Invoke(ctx, TripService_RateTrip_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tripServiceClient) CancelTrip(ctx context.Context, in *CancelTripRequest, opts ...grpc.CallOption) (*CancelTripResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CancelTripResponse)
	err := c.cc.Invoke(ctx, TripService_CancelTrip_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type TripServiceServer interface {
	PreviewTrip(context.Context, *PreviewTripRequest) (*PreviewTripResponse, error)
	CreateTrip(context.Context, *CreateTripRequest) (*CreateTripResponse, error)
	GetTripsByUser(context.Context, *GetTripsRequest) (*TripsResponse, error)
	GetTripsByDriver(context.Context, *GetTripsRequest) (*DriverTripsResponse, error)
	RateTrip(context.Context, *RateTripRequest) (*RateTripResponse, error)
	CancelTrip(context.Context, *CancelTripRequest) (*CancelTripResponse, error)
	mustEmbedUnimplementedTripServiceServer()
}

type UnimplementedTripServiceServer struct{}

func (UnimplementedTripServiceServer) PreviewTrip(context.Context, *PreviewTripRequest) (*PreviewTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PreviewTrip not implemented")
}
func (UnimplementedTripServiceServer) CreateTrip(context.Context, *CreateTripRequest) (*CreateTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTrip not implemented")
}
func (UnimplementedTripServiceServer) GetTripsByUser(context.Context, *GetTripsRequest) (*TripsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTripsByUser not implemented")
}
func (UnimplementedTripServiceServer) GetTripsByDriver(context.Context, *GetTripsRequest) (*DriverTripsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTripsByDriver not implemented")
}
func (UnimplementedTripServiceServer) RateTrip(context.Context, *RateTripRequest) (*RateTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RateTrip not implemented")
}
func (UnimplementedTripServiceServer) CancelTrip(context.Context, *CancelTripRequest) (*CancelTripResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CancelTrip not implemented")
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

func _TripService_GetTripsByUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTripsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).GetTripsByUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_GetTripsByUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).GetTripsByUser(ctx, req.(*GetTripsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TripService_GetTripsByDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTripsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).GetTripsByDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_GetTripsByDriver_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).GetTripsByDriver(ctx, req.(*GetTripsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TripService_RateTrip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RateTripRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).RateTrip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_RateTrip_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).RateTrip(ctx, req.(*RateTripRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TripService_CancelTrip_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CancelTripRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TripServiceServer).CancelTrip(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TripService_CancelTrip_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TripServiceServer).CancelTrip(ctx, req.(*CancelTripRequest))
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
		{
			MethodName: "GetTripsByUser",
			Handler:    _TripService_GetTripsByUser_Handler,
		},
		{
			MethodName: "GetTripsByDriver",
			Handler:    _TripService_GetTripsByDriver_Handler,
		},
		{
			MethodName: "RateTrip",
			Handler:    _TripService_RateTrip_Handler,
		},
		{
			MethodName: "CancelTrip",
			Handler:    _TripService_CancelTrip_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/trip.proto",
}
