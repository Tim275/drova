// Package schema provides a Confluent-compatible Schema Registry client and
// Avro codec for encoding/decoding Kafka messages with schema validation.
//
// Wire format (Confluent Schema Registry protocol):
//   [0x00][4-byte big-endian schema ID][avro bytes]
//
// Usage:
//
//	client := schema.NewRegistryClient("http://schema-registry:8081")
//	id, err := client.Register("trip.event.created-value", schema)
//	encoded, err := schema.Encode(id, schema, &myEvent)
//	err = schema.Decode(encoded, schema, &myEvent)
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/hamba/avro/v2"
)

// RegistryClient is a thread-safe Confluent Schema Registry HTTP client
// with in-memory caching of schema IDs to avoid repeated round-trips.
type RegistryClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	byID       map[int]avro.Schema
	bySubject  map[string]int // subject → schema ID
}

func NewRegistryClient(baseURL string) *RegistryClient {
	return &RegistryClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		byID:       make(map[int]avro.Schema),
		bySubject:  make(map[string]int),
	}
}

type registerRequest struct {
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType,omitempty"`
}

type registerResponse struct {
	ID int `json:"id"`
}

type schemaResponse struct {
	Schema string `json:"schema"`
}

// Register registers a schema under the given subject and returns its schema ID.
// Subsequent calls with the same subject are cached locally (idempotent on the registry side).
func (c *RegistryClient) Register(subject string, schema avro.Schema) (int, error) {
	c.mu.RLock()
	if id, ok := c.bySubject[subject]; ok {
		c.mu.RUnlock()
		return id, nil
	}
	c.mu.RUnlock()

	body, err := json.Marshal(registerRequest{Schema: schema.String()})
	if err != nil {
		return 0, fmt.Errorf("marshal schema: %w", err)
	}

	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)
	resp, err := c.httpClient.Post(url, "application/vnd.schemaregistry.v1+json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("schema registry request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("schema registry %d: %s", resp.StatusCode, b)
	}

	var reg registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	c.mu.Lock()
	c.bySubject[subject] = reg.ID
	c.byID[reg.ID] = schema
	c.mu.Unlock()

	return reg.ID, nil
}

// GetByID fetches a schema by its numeric ID. Results are cached after first fetch.
func (c *RegistryClient) GetByID(id int) (avro.Schema, error) {
	c.mu.RLock()
	if s, ok := c.byID[id]; ok {
		c.mu.RUnlock()
		return s, nil
	}
	c.mu.RUnlock()

	url := fmt.Sprintf("%s/schemas/ids/%d", c.baseURL, id)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}
	defer resp.Body.Close()

	var sr schemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decode schema response: %w", err)
	}

	parsed, err := avro.Parse(sr.Schema)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}

	c.mu.Lock()
	c.byID[id] = parsed
	c.mu.Unlock()

	return parsed, nil
}
