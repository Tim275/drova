package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const tripsIndex = "trips"

var indexMapping = []byte(`{
  "mappings": {
    "properties": {
      "trip_id":         {"type": "keyword"},
      "user_id":         {"type": "keyword"},
      "driver_id":       {"type": "keyword"},
      "status":          {"type": "keyword"},
      "package_slug":    {"type": "keyword"},
      "price_cents":     {"type": "long"},
      "distance_meters": {"type": "integer"},
      "pickup_address":  {"type": "text"},
      "dropoff_address": {"type": "text"},
      "pickup":          {"type": "geo_point"},
      "created_at":      {"type": "date"},
      "completed_at":    {"type": "date"}
    }
  }
}`)

type ES struct {
	baseURL string
	http    *http.Client
}

func NewES(baseURL string) *ES {
	return &ES{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
}

func (e *ES) EnsureIndex(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, e.baseURL+"/"+tripsIndex, nil)
	if err != nil {
		return err
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return e.do(ctx, http.MethodPut, "/"+tripsIndex, indexMapping)
}

func (e *ES) IndexTrip(ctx context.Context, id string, doc any) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return e.do(ctx, http.MethodPut, "/"+tripsIndex+"/_doc/"+id, body)
}

func (e *ES) UpdateTrip(ctx context.Context, id string, fields map[string]any) error {
	body, err := json.Marshal(map[string]any{"doc": fields, "doc_as_upsert": true})
	if err != nil {
		return err
	}
	return e.do(ctx, http.MethodPost, "/"+tripsIndex+"/_update/"+id, body)
}

func (e *ES) do(ctx context.Context, method, path string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, e.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("es %s %s: status %d: %s", method, path, resp.StatusCode, string(body))
	}
	return nil
}
