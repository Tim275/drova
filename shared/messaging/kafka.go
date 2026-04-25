package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"drova/shared/retry"
	"drova/shared/tracing"

	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, msg []byte) error

type Kafka struct {
	brokers []string
	writer  *kafka.Writer
}

func NewKafka(brokers []string) *Kafka {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}

	return &Kafka{brokers: brokers, writer: writer}
}

func (k *Kafka) EnsureTopics(topics ...string) error {
	cfg := retry.Config{
		MaxRetries:  10,
		InitialWait: 1 * time.Second,
		MaxWait:     5 * time.Second,
	}

	return retry.WithBackoff(context.Background(), cfg, func() error {
		conn, err := kafka.Dial("tcp", k.brokers[0])
		if err != nil {
			return fmt.Errorf("dial kafka: %w", err)
		}
		defer conn.Close()

		controller, err := conn.Controller()
		if err != nil {
			return fmt.Errorf("get controller: %w", err)
		}

		ctrlConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
		if err != nil {
			return fmt.Errorf("dial controller: %w", err)
		}
		defer ctrlConn.Close()

		configs := make([]kafka.TopicConfig, 0, len(topics))
		for _, t := range topics {
			configs = append(configs, kafka.TopicConfig{
				Topic:             t,
				NumPartitions:     1,
				ReplicationFactor: 1,
			})
		}

		if err := ctrlConn.CreateTopics(configs...); err != nil {
			return fmt.Errorf("create topics: %w", err)
		}

		log.Printf("ensured %d kafka topics exist", len(topics))
		return nil
	})
}

func (k *Kafka) PublishMessage(ctx context.Context, topic string, message []byte) error {
	ctx, end := tracing.StartKafkaProducerSpan(ctx, topic, "")
	defer end()

	headers := make(map[string][]byte)
	tracing.InjectKafkaHeaders(ctx, headers)

	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for key, val := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: key, Value: val})
	}

	err := k.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Value:   message,
		Headers: kafkaHeaders,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}
	return nil
}

func (k *Kafka) ConsumeMessages(ctx context.Context, topic, groupID string, handler MessageHandler) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.brokers,
		Topic:   topic,
		GroupID: groupID,
	})
	defer reader.Close()

	retryCfg := retry.DefaultConfig()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("error fetching message from %s: %v", topic, err)
			continue
		}

		headers := make(map[string][]byte, len(msg.Headers))
		for _, h := range msg.Headers {
			headers[h.Key] = h.Value
		}
		msgCtx := tracing.ExtractKafkaHeaders(ctx, headers)
		msgCtx, end := tracing.StartKafkaConsumerSpan(msgCtx, topic, groupID)

		handlerErr := retry.WithBackoff(msgCtx, retryCfg, func() error {
			return handler(msgCtx, msg.Value)
		})
		end()

		if handlerErr != nil {
			log.Printf("handler failed after %d retries for topic %s: %v — sending to DLQ",
				retryCfg.MaxRetries, topic, handlerErr)

			if dlqErr := k.publishToDLQ(ctx, msg, handlerErr, retryCfg.MaxRetries); dlqErr != nil {
				log.Printf("failed to publish to DLQ: %v", dlqErr)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("error committing message: %v", err)
		}
	}
}

func (k *Kafka) publishToDLQ(ctx context.Context, original kafka.Message, failureErr error, retryCount int) error {
	dlq := DLQMessage{
		OriginalTopic: original.Topic,
		OriginalKey:   string(original.Key),
		FailureReason: failureErr.Error(),
		RetryCount:    retryCount,
		FailedAt:      time.Now().UTC(),
		Payload:       original.Value,
	}

	payload, err := json.Marshal(dlq)
	if err != nil {
		return fmt.Errorf("marshal DLQ message: %w", err)
	}

	return k.PublishMessage(ctx, TopicDeadLetterQueue, payload)
}

func (k *Kafka) Close() {
	k.writer.Close()
}
