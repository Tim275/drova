//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Seed-User existieren bereits in der DB (user-service/store/seed.go)
// Kein Email-Aktivierung nötig — direkt verwendbar
const (
	seedRider    = "rider@drova.local"
	seedDriver   = "driver@drova.local"
	seedPassword = "Test1234!"
)

var baseURL = func() string {
	if u := os.Getenv("E2E_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8081"
}()

var httpClient = &http.Client{Timeout: 10 * time.Second}

// login gibt einen JWT Token für den angegebenen User zurück
func login(t *testing.T, email, password string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", baseURL+"/v1/auth/token", nil)
	req.SetBasicAuth(email, password)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("login decode failed: %v", err)
	}
	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatal("login: no token in response")
	}
	return token
}

func authHeader(token string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	return h
}

func get(t *testing.T, path string, headers http.Header) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	for k, v := range headers {
		req.Header[k] = v
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	return resp
}

func post(t *testing.T, path, body string, headers http.Header) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", baseURL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header[k] = v
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s failed: %v", path, err)
	}
	return resp
}

// TestHealth — api-gateway antwortet
func TestHealth(t *testing.T) {
	resp := get(t, "/health", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health: want 200, got %d", resp.StatusCode)
	}
	t.Log("✓ /health ok")
}

// TestReadyz — Kafka-Verbindung steht
func TestReadyz(t *testing.T) {
	resp := get(t, "/readyz", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz: want 200, got %d", resp.StatusCode)
	}
	t.Log("✓ /readyz ok")
}

// TestLogin — Seed-User kann sich einloggen, JWT wird ausgestellt
func TestLogin(t *testing.T) {
	token := login(t, seedRider, seedPassword)
	if len(token) < 10 {
		t.Fatalf("token too short: %q", token)
	}
	t.Log("✓ login ok, token received")
}

// TestGetMe — JWT ist gültig, user-service antwortet
func TestGetMe(t *testing.T) {
	token := login(t, seedRider, seedPassword)
	resp := get(t, "/v1/users/me", authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("getme: want 200, got %d", resp.StatusCode)
	}
	var user map[string]any
	json.NewDecoder(resp.Body).Decode(&user)
	if user["email"] != seedRider {
		t.Fatalf("getme: wrong email %v", user["email"])
	}
	t.Log("✓ /v1/users/me ok, email matches")
}

// TestTripHistory — trip-service erreichbar (leere Liste ist ok)
func TestTripHistory(t *testing.T) {
	token := login(t, seedRider, seedPassword)
	resp := get(t, "/trips/history", authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trip history: want 200, got %d", resp.StatusCode)
	}
	t.Log("✓ /trips/history ok")
}

// TestTripPreview — trip-service + Mapbox erreichbar
// Wird übersprungen wenn MAPBOX_TOKEN nicht gesetzt (kein Secret in PR)
func TestTripPreview(t *testing.T) {
	if os.Getenv("MAPBOX_TOKEN") == "" {
		t.Skip("MAPBOX_TOKEN not set — skipping trip preview")
	}
	token := login(t, seedRider, seedPassword)
	body := `{"pickup":"Alexanderplatz, Berlin","dropoff":"Brandenburger Tor, Berlin"}`
	resp := post(t, "/trip/preview", body, authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trip preview: want 200, got %d", resp.StatusCode)
	}
	t.Log("✓ /trip/preview ok")
}

// TestDriverLogin — Driver Seed-User funktioniert auch
func TestDriverLogin(t *testing.T) {
	token := login(t, seedDriver, seedPassword)
	resp := get(t, "/v1/users/me", authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("driver getme: want 200, got %d", resp.StatusCode)
	}
	var user map[string]any
	json.NewDecoder(resp.Body).Decode(&user)
	if user["role"] != "driver" {
		t.Fatalf("expected role=driver, got %v", user["role"])
	}
	t.Log("✓ driver login + role check ok")
}
