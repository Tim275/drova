package tracing

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
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
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.DeploymentEnvironmentKey.String(cfg.Environment),
	)

	hasOTLP := cfg.OtelCollectorEndpoint != ""

	// Metrics: expose a Prometheus pull endpoint (/metrics) always; additionally
	// push to the OTLP collector when one is configured.
	metricOpts := []metric.Option{metric.WithResource(res)}
	if promExp, perr := otelprom.New(); perr == nil {
		metricOpts = append(metricOpts, metric.WithReader(promExp))
	}
	if hasOTLP {
		if metricExp, merr := otlpmetrichttp.New(context.Background(),
			otlpmetrichttp.WithEndpoint(cfg.OtelCollectorEndpoint),
			otlpmetrichttp.WithInsecure(),
		); merr == nil {
			metricOpts = append(metricOpts, metric.WithReader(metric.NewPeriodicReader(metricExp)))
		}
	}
	mp := metric.NewMeterProvider(metricOpts...)
	otel.SetMeterProvider(mp)
	_ = runtime.Start(runtime.WithMinimumReadMemStatsInterval(15))

	// Traces + logs: only when an OTLP collector endpoint is configured.
	var tp *sdktrace.TracerProvider
	var lp *sdklog.LoggerProvider
	if hasOTLP {
		traceExp, err := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(cfg.OtelCollectorEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return func(ctx context.Context) { mp.Shutdown(ctx) }, err
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		if logExp, lerr := otlploghttp.New(context.Background(),
			otlploghttp.WithEndpoint(cfg.OtelCollectorEndpoint),
			otlploghttp.WithInsecure(),
		); lerr == nil {
			lp = sdklog.NewLoggerProvider(
				sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
				sdklog.WithResource(res),
			)
			global.SetLoggerProvider(lp)
		}
	}

	return func(ctx context.Context) {
		if tp != nil {
			tp.Shutdown(ctx)
		}
		mp.Shutdown(ctx)
		if lp != nil {
			lp.Shutdown(ctx)
		}
	}, nil
}

func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
