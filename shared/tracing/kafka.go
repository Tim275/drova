package tracing

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type KafkaHeadersCarrier map[string][]byte

func (c KafkaHeadersCarrier) Get(key string) string {
	if val, ok := c[key]; ok {
		return string(val)
	}
	return ""
}

func (c KafkaHeadersCarrier) Set(key string, val string) {
	c[key] = []byte(val)
}

func (c KafkaHeadersCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

func InjectKafkaHeaders(ctx context.Context, headers map[string][]byte) {
	otel.GetTextMapPropagator().Inject(ctx, KafkaHeadersCarrier(headers))
}

func ExtractKafkaHeaders(ctx context.Context, headers map[string][]byte) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, KafkaHeadersCarrier(headers))
}

func StartKafkaProducerSpan(ctx context.Context, topic, ownerID string) (context.Context, func()) {
	tracer := otel.Tracer("kafka.producer")
	ctx, span := tracer.Start(ctx, "kafka.publish")
	span.SetAttributes(
		attribute.String("messaging.destination", topic),
		attribute.String("messaging.system", "kafka"),
		attribute.String("owner_id", ownerID),
	)
	return ctx, func() { span.End() }
}

func StartKafkaConsumerSpan(ctx context.Context, topic, groupID string) (context.Context, func()) {
	tracer := otel.Tracer("kafka.consumer")
	ctx, span := tracer.Start(ctx, "kafka.consume")
	span.SetAttributes(
		attribute.String("messaging.source", topic),
		attribute.String("messaging.consumer_group", groupID),
		attribute.String("messaging.system", "kafka"),
	)
	return ctx, func() { span.End() }
}

// Kafka metrics. The global MeterProvider is wired in InitTracer; instruments are
// created lazily on first record so the provider is guaranteed to be set (records
// only happen at runtime). Prometheus sees: messaging_publish_messages_total,
// messaging_consume_messages_total, messaging_process_duration_seconds.
var (
	kafkaMetricsOnce sync.Once
	kafkaPublished   metric.Int64Counter
	kafkaConsumed    metric.Int64Counter
	kafkaProcessTime metric.Float64Histogram
)

func initKafkaMetrics() {
	meter := otel.Meter("drova/kafka")
	kafkaPublished, _ = meter.Int64Counter("messaging.publish.messages",
		metric.WithDescription("Kafka messages published"))
	kafkaConsumed, _ = meter.Int64Counter("messaging.consume.messages",
		metric.WithDescription("Kafka messages consumed"))
	kafkaProcessTime, _ = meter.Float64Histogram("messaging.process.duration",
		metric.WithDescription("Kafka consumer handler processing time"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10))
}

func kafkaStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// RecordKafkaPublish counts a publish attempt (status=ok|error) by topic.
func RecordKafkaPublish(ctx context.Context, topic string, err error) {
	kafkaMetricsOnce.Do(initKafkaMetrics)
	kafkaPublished.Add(ctx, 1, metric.WithAttributes(
		attribute.String("topic", topic),
		attribute.String("status", kafkaStatus(err)),
	))
}

// RecordKafkaConsume counts a consumed message and records handler latency,
// labelled by topic, consumer group and status=ok|error.
func RecordKafkaConsume(ctx context.Context, topic, group string, dur time.Duration, err error) {
	kafkaMetricsOnce.Do(initKafkaMetrics)
	attrs := metric.WithAttributes(
		attribute.String("topic", topic),
		attribute.String("group", group),
		attribute.String("status", kafkaStatus(err)),
	)
	kafkaConsumed.Add(ctx, 1, attrs)
	kafkaProcessTime.Record(ctx, dur.Seconds(), attrs)
}
