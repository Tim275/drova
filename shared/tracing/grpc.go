package tracing

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

func WithTracingInterceptors() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
}

func DialOptionsWithTracing() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
}

// DriverDialOptionsWithTracing excludes StreamLocation from tracing.
// StreamLocation is a long-lived stream that always ends CANCELLED on driver
// disconnect — every span would be a false error with 25+ min duration.
func DriverDialOptionsWithTracing() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(
			otelgrpc.WithFilter(func(info *stats.RPCTagInfo) bool {
				return info.FullMethodName != "/driver.DriverService/StreamLocation"
			}),
		)),
	}
}
