package indexer

import (
	"encoding/json"
	"testing"

	"drova/shared/messaging"
)

func TestIndexMapping_IsValidJSON(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(indexMapping, &m); err != nil {
		t.Fatalf("indexMapping is not valid JSON: %v", err)
	}
	mappings, ok := m["mappings"].(map[string]any)
	if !ok {
		t.Fatal("indexMapping missing 'mappings' object")
	}
	if _, ok := mappings["properties"]; !ok {
		t.Fatal("indexMapping missing 'mappings.properties'")
	}
}

func TestDecode_ExtractsTripCreatedFromEnvelope(t *testing.T) {
	inner := messaging.TripCreatedEvent{
		TripID:      "t-123",
		UserID:      "u-456",
		PackageSlug: "economy",
		PriceCents:  1500,
	}
	innerBytes, _ := json.Marshal(inner)
	envelope := messaging.KafkaMessage{Data: innerBytes}
	envelopeBytes, _ := json.Marshal(envelope)

	var got messaging.TripCreatedEvent
	if err := decode(envelopeBytes, &got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.TripID != "t-123" || got.UserID != "u-456" || got.PriceCents != 1500 {
		t.Errorf("decoded mismatch: %+v", got)
	}
}

func TestDecode_RejectsInvalidJSON(t *testing.T) {
	var out messaging.TripCreatedEvent
	if err := decode([]byte("not-json"), &out); err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}
