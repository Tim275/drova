package messaging

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
	"time"

	"drova/shared/schema"
	"drova/shared/tracing"

	"github.com/hamba/avro/v2"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

type MessageHandler func(ctx context.Context, msg []byte) error

type Kafka struct {
	brokers  []string
	writer   *kafka.Writer
	dialer   *kafka.Dialer
	registry *schema.RegistryClient
	wg       sync.WaitGroup
	handlers sync.Map // map[originalTopic string]MessageHandler — used by retry consumers
}

func NewKafka(brokers []string) *Kafka {
	dialer := kafka.DefaultDialer
	var transport kafka.RoundTripper

	var tlsCfg *tls.Config
	if caPath := os.Getenv("KAFKA_TLS_CA_CERT"); caPath != "" {
		ca, err := os.ReadFile(caPath) //nolint:gosec
		if err != nil {
			log.Fatalf("kafka: read CA cert %s: %v", caPath, err) //nolint:gosec
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(ca)
		tlsCfg = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
		log.Printf("kafka: TLS enabled (CA: %s)", caPath) //nolint:gosec
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
			log.Printf("kafka: SASL/SCRAM-SHA-512 enabled for user %s", saslUser) //nolint:gosec
		} else {
			m := plain.Mechanism{Username: saslUser, Password: saslPass}
			transport = &kafka.Transport{SASL: m, TLS: tlsCfg}
			dialer = &kafka.Dialer{Timeout: 10 * time.Second, DualStack: true, SASLMechanism: m, TLS: tlsCfg}
			log.Printf("kafka: SASL/PLAIN enabled for user %s", saslUser) //nolint:gosec
		}
	} else if tlsCfg != nil {
		transport = &kafka.Transport{TLS: tlsCfg}
		dialer = &kafka.Dialer{Timeout: 10 * time.Second, DualStack: true, TLS: tlsCfg}
	}

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Murmur2Balancer{},
		AllowAutoTopicCreation: false,
		Transport:              transport,
	}

	k := &Kafka{brokers: brokers, writer: writer, dialer: dialer}

	if url := os.Getenv("SCHEMA_REGISTRY_URL"); url != "" {
		k.registry = schema.NewRegistryClient(url)
		log.Printf("schema registry: client initialised (%s)", url) //nolint:gosec
	}

	return k
}

func (k *Kafka) EnsureTopics(topics ...string) error {
	cfg := retryConfig{
		MaxRetries:  10,
		InitialWait: 1 * time.Second,
		MaxWait:     5 * time.Second,
	}

	return withBackoff(context.Background(), cfg, func() error {
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

func (k *Kafka) RegisterSchemas(ctx context.Context, subjects map[string]avro.Schema) {
	if k.registry == nil {
		return
	}
	for subject, s := range subjects {
		id, err := k.registry.Register(subject, s)
		if err != nil {
			log.Printf("schema registry: register %s: %v", subject, err)
			continue
		}
		log.Printf("schema registry: %s registered (id=%d)", subject, id)
	}
}

func (k *Kafka) PublishAvro(ctx context.Context, topic string, schemaID int, avroSchema avro.Schema, v any) error {
	payload, err := schema.Encode(schemaID, avroSchema, v)
	if err != nil {
		return fmt.Errorf("avro encode: %w", err)
	}
	return k.PublishMessage(ctx, topic, payload)
}

// partitionKey peeks well-known entity IDs from a JSON event payload so related
// messages (same trip, then owner/user/driver/rider) hash to the same Kafka
// partition and stay ordered once topics have more than one partition. Returns
// nil for payloads without a known ID, letting the balancer round-robin them.
func partitionKey(message []byte) []byte {
	var probe struct {
		TripID   string `json:"trip_id"`
		OwnerID  string `json:"owner_id"`
		UserID   string `json:"user_id"`
		DriverID string `json:"driver_id"`
		RiderID  string `json:"rider_id"`
	}
	if err := json.Unmarshal(message, &probe); err != nil {
		return nil
	}
	switch {
	case probe.TripID != "":
		return []byte(probe.TripID)
	case probe.OwnerID != "":
		return []byte(probe.OwnerID)
	case probe.UserID != "":
		return []byte(probe.UserID)
	case probe.DriverID != "":
		return []byte(probe.DriverID)
	case probe.RiderID != "":
		return []byte(probe.RiderID)
	}
	return nil
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
		Key:     partitionKey(message),
		Value:   message,
		Headers: kafkaHeaders,
	})
	tracing.RecordKafkaPublish(ctx, topic, err)
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}
	return nil
}

// ConsumeMessages launches a background goroutine that consumes messages from
// the given topic. Call Wait() during shutdown to block until all consumers exit.
// The handler is also registered for retry consumers — failed messages routed
// through retry topics will be re-attempted by StartRetryConsumers.
func (k *Kafka) ConsumeMessages(ctx context.Context, topic, groupID string, handler MessageHandler) {
	k.handlers.Store(topic, handler)
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		k.consumeLoop(ctx, topic, groupID, handler)
	}()
}

// StartRetryConsumers launches retry consumers for drova.retry.{1,2,3} and a
// DLQ logger. Call this once after all ConsumeMessages registrations.
// serviceID is used as the consumer group prefix (e.g. "api-gateway").
func (k *Kafka) StartRetryConsumers(ctx context.Context, serviceID string) {
	retryTopics := []string{TopicRetry1, TopicRetry2, TopicRetry3}
	nextAttempts := []int{2, 3, 4} // retry-1 failure → attempt 2 → retry-2, etc.

	for i, topic := range retryTopics {
		t := topic
		nextAttempt := nextAttempts[i]
		k.wg.Add(1)
		go func() {
			defer k.wg.Done()
			k.consumeRetryLoop(ctx, t, serviceID+"-retry", nextAttempt)
		}()
	}

	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		k.consumeDLQLoop(ctx, serviceID+"-dlq-monitor")
	}()
}

// Wait blocks until all consumer goroutines started by ConsumeMessages have exited.
// Call this after cancelling the context to ensure a clean shutdown.
func (k *Kafka) Wait() {
	k.wg.Wait()
}

func (k *Kafka) consumeLoop(ctx context.Context, topic, groupID string, handler MessageHandler) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.brokers,
		Topic:       topic,
		GroupID:     groupID,
		Dialer:      k.dialer,
		StartOffset: kafka.FirstOffset,
	})
	defer reader.Close()

	const (
		fetchBackoffInit = 1 * time.Second
		fetchBackoffMax  = 30 * time.Second
	)
	fetchBackoff := fetchBackoffInit

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			jitter := time.Duration(rand.Int64N(int64(fetchBackoff / 4))) // #nosec G404
			wait := fetchBackoff + jitter
			log.Printf("error fetching message from %s: %v (retry in %v)", topic, err, wait)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			if fetchBackoff < fetchBackoffMax {
				fetchBackoff = min(fetchBackoff*2, fetchBackoffMax)
			}
			continue
		}
		fetchBackoff = fetchBackoffInit

		headers := make(map[string][]byte, len(msg.Headers))
		for _, h := range msg.Headers {
			headers[h.Key] = h.Value
		}
		msgCtx := tracing.ExtractKafkaHeaders(ctx, headers)
		msgCtx, end := tracing.StartKafkaConsumerSpan(msgCtx, topic, groupID)
		start := time.Now()
		handlerErr := handler(msgCtx, msg.Value)
		end()
		tracing.RecordKafkaConsume(msgCtx, topic, groupID, time.Since(start), handlerErr)

		if handlerErr != nil {
			log.Printf("handler failed for topic %s: %v — routing to retry-1", topic, handlerErr)
			if retryErr := k.publishToRetry(ctx, msg, groupID, 1, handlerErr); retryErr != nil {
				// publishToRetry can fail if the payload is not valid JSON (e.g. a completely
				// malformed message). Commit and skip rather than looping forever.
				log.Printf("failed to publish to retry-1 for topic %s: %v — committing to avoid infinite redelivery", topic, retryErr)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("error committing message: %v", err)
		}
	}
}

func (k *Kafka) consumeRetryLoop(ctx context.Context, retryTopic, groupID string, nextAttempt int) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.brokers,
		Topic:   retryTopic,
		GroupID: groupID,
		Dialer:  k.dialer,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		var retryMsg RetryMessage
		if err := json.Unmarshal(msg.Value, &retryMsg); err != nil {
			log.Printf("retry consumer: unmarshal failed on %s: %v — discarding", retryTopic, err)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		// Only handle topics this service has registered a handler for.
		h, ok := k.handlers.Load(retryMsg.OriginalTopic)
		if !ok {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}
		handler := h.(MessageHandler) //nolint:forcetypeassert

		// Wait until the scheduled retry time.
		if wait := time.Until(retryMsg.NextAttemptAt); wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}

		if handlerErr := handler(ctx, retryMsg.Payload); handlerErr != nil {
			log.Printf("retry attempt %d failed for topic %s: %v", retryMsg.AttemptNumber, retryMsg.OriginalTopic, handlerErr)
			original := kafka.Message{
				Topic: retryMsg.OriginalTopic,
				Key:   []byte(retryMsg.OriginalKey),
				Value: retryMsg.Payload,
			}
			if pubErr := k.publishToRetry(ctx, original, retryMsg.OriginalGroupID, nextAttempt, handlerErr); pubErr != nil {
				log.Printf("failed to publish attempt %d: %v — skipping commit", nextAttempt, pubErr)
				continue
			}
		}

		_ = reader.CommitMessages(ctx, msg)
	}
}

func (k *Kafka) consumeDLQLoop(ctx context.Context, groupID string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: k.brokers,
		Topic:   TopicDeadLetterQueue,
		GroupID: groupID,
		Dialer:  k.dialer,
	})
	defer reader.Close()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		var dlqMsg DLQMessage
		if err := json.Unmarshal(msg.Value, &dlqMsg); err != nil {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		// Only log DLQ messages relevant to this service.
		if _, ok := k.handlers.Load(dlqMsg.OriginalTopic); !ok {
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		log.Printf("DLQ: topic=%s retries=%d reason=%q failedAt=%s payload=%s",
			dlqMsg.OriginalTopic, dlqMsg.RetryCount, dlqMsg.FailureReason,
			dlqMsg.FailedAt.Format(time.RFC3339), string(dlqMsg.Payload))

		_ = reader.CommitMessages(ctx, msg)
	}
}

// publishToRetry routes a failed message to the appropriate retry topic.
// attempt 1 → drova.retry.1 (1 min delay)
// attempt 2 → drova.retry.2 (5 min delay)
// attempt 3 → drova.retry.3 (30 min delay)
// attempt 4+ → dead.letter.queue
func (k *Kafka) publishToRetry(ctx context.Context, original kafka.Message, groupID string, attempt int, failureErr error) error {
	retryTopics := []string{TopicRetry1, TopicRetry2, TopicRetry3}
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}

	if attempt > len(retryTopics) {
		dlq := DLQMessage{
			OriginalTopic: original.Topic,
			OriginalKey:   string(original.Key),
			FailureReason: failureErr.Error(),
			RetryCount:    attempt - 1,
			FailedAt:      time.Now().UTC(),
			Payload:       original.Value,
		}
		payload, err := json.Marshal(dlq)
		if err != nil {
			return fmt.Errorf("marshal DLQ message: %w", err)
		}
		log.Printf("max retries exceeded for topic %s — sending to DLQ", original.Topic)
		return k.PublishMessage(ctx, TopicDeadLetterQueue, payload)
	}

	retryMsg := RetryMessage{
		OriginalTopic:   original.Topic,
		OriginalKey:     string(original.Key),
		OriginalGroupID: groupID,
		Payload:         original.Value,
		AttemptNumber:   attempt,
		NextAttemptAt:   time.Now().Add(delays[attempt-1]),
		FailureReason:   failureErr.Error(),
	}
	payload, err := json.Marshal(retryMsg)
	if err != nil {
		return fmt.Errorf("marshal retry message: %w", err)
	}
	return k.PublishMessage(ctx, retryTopics[attempt-1], payload)
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
