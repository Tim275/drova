package messaging

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"drova/shared/retry"
	"drova/shared/tracing"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type MessageHandler func(ctx context.Context, msg []byte) error

type Kafka struct {
	brokers []string
	writer  *kafka.Writer
	dialer  *kafka.Dialer
}

func NewKafka(brokers []string) *Kafka {
	dialer := kafka.DefaultDialer
	var transport kafka.RoundTripper

	var tlsCfg *tls.Config
	if caPath := os.Getenv("KAFKA_TLS_CA_CERT"); caPath != "" {
		ca, err := os.ReadFile(caPath)
		if err != nil {
			log.Fatalf("kafka: read CA cert %s: %v", caPath, err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(ca)
		tlsCfg = &tls.Config{RootCAs: pool}
		log.Printf("kafka: TLS enabled (CA: %s)", caPath)
	}

	saslUser := os.Getenv("KAFKA_SASL_USERNAME")
	saslPass := os.Getenv("KAFKA_SASL_PASSWORD")
	if saslUser != "" && saslPass != "" {
		if os.Getenv("KAFKA_SASL_MECHANISM") == "SCRAM-SHA-512" {
			m, err := scram.Mechanism(scram.SHA512, saslUser, saslPass)
			if err != nil {
				log.Fatalf("kafka: SCRAM-SHA-512 init: %v", err)
			}
			transport = &kafka.Transport{SASL: m, TLS: tlsCfg}
			dialer = &kafka.Dialer{Timeout: 10 * time.Second, DualStack: true, SASLMechanism: m, TLS: tlsCfg}
			log.Printf("kafka: SASL/SCRAM-SHA-512 enabled for user %s", saslUser)
		} else {
			m := plain.Mechanism{Username: saslUser, Password: saslPass}
			transport = &kafka.Transport{SASL: m, TLS: tlsCfg}
			dialer = &kafka.Dialer{Timeout: 10 * time.Second, DualStack: true, SASLMechanism: m, TLS: tlsCfg}
			log.Printf("kafka: SASL/PLAIN enabled for user %s", saslUser)
		}
	} else if tlsCfg != nil {
		transport = &kafka.Transport{TLS: tlsCfg}
		dialer = &kafka.Dialer{Timeout: 10 * time.Second, DualStack: true, TLS: tlsCfg}
	}

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: false,
		Transport:              transport,
	}

	return &Kafka{brokers: brokers, writer: writer, dialer: dialer}
}

func (k *Kafka) EnsureTopics(topics ...string) error {
	cfg := retry.Config{
		MaxRetries:  10,
		InitialWait: 1 * time.Second,
		MaxWait:     5 * time.Second,
	}

	return retry.WithBackoff(context.Background(), cfg, func() error {
		conn, err := k.dialer.Dial("tcp", k.brokers[0])
		if err != nil {
			return fmt.Errorf("dial kafka: %w", err)
		}
		defer conn.Close()

		controller, err := conn.Controller()
		if err != nil {
			return fmt.Errorf("get controller: %w", err)
		}

		ctrlConn, err := k.dialer.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
		if err != nil {
			return fmt.Errorf("dial controller: %w", err)
		}
		defer ctrlConn.Close()

		rf := 1
		if v := os.Getenv("KAFKA_REPLICATION_FACTOR"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				rf = n
			}
		}

		configs := make([]kafka.TopicConfig, 0, len(topics))
		for _, t := range topics {
			configs = append(configs, kafka.TopicConfig{
				Topic:             t,
				NumPartitions:     1,
				ReplicationFactor: rf,
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
		Dialer:  k.dialer,
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
				log.Printf("failed to publish to DLQ: %v — skipping commit to avoid message loss", dlqErr)
				continue // do not commit: let Kafka redeliver so we can retry DLQ publish
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

func (k *Kafka) Ping(ctx context.Context) error {
	conn, err := k.dialer.DialContext(ctx, "tcp", k.brokers[0])
	if err != nil {
		return fmt.Errorf("kafka ping: %w", err)
	}
	conn.Close()
	return nil
}

func (k *Kafka) Close() {
	k.writer.Close()
}
