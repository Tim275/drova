package tracing

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName           string
	Environment           string
	OtelCollectorEndpoint string
}

func InitTracer(cfg Config) (func(context.Context), error) {
	noop := func(context.Context) {}
	if cfg.OtelCollectorEndpoint == "" {
		return noop, nil
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.DeploymentEnvironmentKey.String(cfg.Environment),
	)

	traceExp, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(cfg.OtelCollectorEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return noop, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var mp *metric.MeterProvider
	metricExp, err := otlpmetrichttp.New(context.Background(),
		otlpmetrichttp.WithEndpoint(cfg.OtelCollectorEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err == nil {
		mp = metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(metricExp)),
			metric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		_ = runtime.Start(runtime.WithMinimumReadMemStatsInterval(15))
	}

	var lp *sdklog.LoggerProvider
	logExp, err := otlploghttp.New(context.Background(),
		otlploghttp.WithEndpoint(cfg.OtelCollectorEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err == nil {
		lp = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
			sdklog.WithResource(res),
		)
		global.SetLoggerProvider(lp)
	}

	return func(ctx context.Context) {
		tp.Shutdown(ctx)
		if mp != nil {
			mp.Shutdown(ctx)
		}
		if lp != nil {
			lp.Shutdown(ctx)
		}
	}, nil
}

func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
