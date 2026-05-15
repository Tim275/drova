package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
