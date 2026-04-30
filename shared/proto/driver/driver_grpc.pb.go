
package driver

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const (
	DriverService_RegisterDriver_FullMethodName   = "/driver.DriverService/RegisterDriver"
	DriverService_UnregisterDriver_FullMethodName = "/driver.DriverService/UnregisterDriver"
	DriverService_StreamLocation_FullMethodName   = "/driver.DriverService/StreamLocation"
)

type DriverServiceClient interface {
	RegisterDriver(ctx context.Context, in *RegisterDriverRequest, opts ...grpc.CallOption) (*RegisterDriverResponse, error)
	UnregisterDriver(ctx context.Context, in *RegisterDriverRequest, opts ...grpc.CallOption) (*RegisterDriverResponse, error)
	StreamLocation(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[LocationUpdate, StreamLocationResponse], error)
}

type driverServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDriverServiceClient(cc grpc.ClientConnInterface) DriverServiceClient {
	return &driverServiceClient{cc}
}

func (c *driverServiceClient) RegisterDriver(ctx context.Context, in *RegisterDriverRequest, opts ...grpc.CallOption) (*RegisterDriverResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterDriverResponse)
	err := c.cc.Invoke(ctx, DriverService_RegisterDriver_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *driverServiceClient) UnregisterDriver(ctx context.Context, in *RegisterDriverRequest, opts ...grpc.CallOption) (*RegisterDriverResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RegisterDriverResponse)
	err := c.cc.Invoke(ctx, DriverService_UnregisterDriver_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *driverServiceClient) StreamLocation(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[LocationUpdate, StreamLocationResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &DriverService_ServiceDesc.Streams[0], DriverService_StreamLocation_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[LocationUpdate, StreamLocationResponse]{ClientStream: stream}
	return x, nil
}

type DriverService_StreamLocationClient = grpc.ClientStreamingClient[LocationUpdate, StreamLocationResponse]

type DriverServiceServer interface {
	RegisterDriver(context.Context, *RegisterDriverRequest) (*RegisterDriverResponse, error)
	UnregisterDriver(context.Context, *RegisterDriverRequest) (*RegisterDriverResponse, error)
	StreamLocation(grpc.ClientStreamingServer[LocationUpdate, StreamLocationResponse]) error
	mustEmbedUnimplementedDriverServiceServer()
}

type UnimplementedDriverServiceServer struct{}

func (UnimplementedDriverServiceServer) RegisterDriver(context.Context, *RegisterDriverRequest) (*RegisterDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterDriver not implemented")
}
func (UnimplementedDriverServiceServer) UnregisterDriver(context.Context, *RegisterDriverRequest) (*RegisterDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnregisterDriver not implemented")
}
func (UnimplementedDriverServiceServer) StreamLocation(grpc.ClientStreamingServer[LocationUpdate, StreamLocationResponse]) error {
	return status.Errorf(codes.Unimplemented, "method StreamLocation not implemented")
}
func (UnimplementedDriverServiceServer) mustEmbedUnimplementedDriverServiceServer() {}
func (UnimplementedDriverServiceServer) testEmbeddedByValue()                       {}

type UnsafeDriverServiceServer interface {
	mustEmbedUnimplementedDriverServiceServer()
}

func RegisterDriverServiceServer(s grpc.ServiceRegistrar, srv DriverServiceServer) {
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&DriverService_ServiceDesc, srv)
}

func _DriverService_RegisterDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).RegisterDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DriverService_RegisterDriver_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).RegisterDriver(ctx, req.(*RegisterDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_UnregisterDriver_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterDriverRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DriverServiceServer).UnregisterDriver(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DriverService_UnregisterDriver_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DriverServiceServer).UnregisterDriver(ctx, req.(*RegisterDriverRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DriverService_StreamLocation_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DriverServiceServer).StreamLocation(&grpc.GenericServerStream[LocationUpdate, StreamLocationResponse]{ServerStream: stream})
}

type DriverService_StreamLocationServer = grpc.ClientStreamingServer[LocationUpdate, StreamLocationResponse]

var DriverService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "driver.DriverService",
	HandlerType: (*DriverServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterDriver",
			Handler:    _DriverService_RegisterDriver_Handler,
		},
		{
			MethodName: "UnregisterDriver",
			Handler:    _DriverService_UnregisterDriver_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamLocation",
			Handler:       _DriverService_StreamLocation_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "proto/driver.proto",
}
