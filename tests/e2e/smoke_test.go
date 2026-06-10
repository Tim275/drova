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

// ── MAPBOX-ABHÄNGIGE TESTS — DEAKTIVIERT ──────────────────────────────────
// Mapbox wird evtl. durch einen anderen Routing-Provider ersetzt.
// Zum Re-Enablen: dieses Block-Kommentar entfernen + den Inject-Step
// in .github/workflows/e2e-tests.yaml wieder aktivieren.
/*
// getUserID liefert die User-ID des eingeloggten Users (für preview/start Payloads).
func getUserID(t *testing.T, token string) string {
	t.Helper()
	resp := get(t, "/v1/users/me", authHeader(token))
	defer resp.Body.Close()
	var user map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("getme decode: %v", err)
	}
	switch id := user["id"].(type) {
	case float64:
		return fmt.Sprintf("%.0f", id)
	case string:
		return id
	default:
		t.Fatalf("getme: unexpected id type %T", user["id"])
		return ""
	}
}

func previewBody(userID string) string {
	return fmt.Sprintf(`{
		"userID": %q,
		"pickup":      {"latitude": 51.2277, "longitude": 6.7735},
		"destination": {"latitude": 51.2180, "longitude": 6.7940},
		"pickupAddress":  "Königsallee, Düsseldorf",
		"dropoffAddress": "Hauptbahnhof, Düsseldorf"
	}`, userID)
}

// TestTripPreview — trip-service + Mapbox Directions erreichbar
// Wird übersprungen wenn MAPBOX_TOKEN nicht gesetzt
func TestTripPreview(t *testing.T) {
	if os.Getenv("MAPBOX_TOKEN") == "" {
		t.Skip("MAPBOX_TOKEN not set — skipping trip preview")
	}
	token := login(t, seedRider, seedPassword)
	userID := getUserID(t, token)

	resp := post(t, "/trip/preview", previewBody(userID), authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trip preview: want 200, got %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			Route     json.RawMessage  `json:"route"`
			RideFares []map[string]any `json:"rideFares"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("preview decode: %v", err)
	}
	if len(out.Data.Route) == 0 {
		t.Fatal("preview: no route in response")
	}
	if len(out.Data.RideFares) == 0 {
		t.Fatal("preview: no rideFares in response")
	}
	t.Logf("✓ /trip/preview ok (%d fare options)", len(out.Data.RideFares))
}

// TestCreateTrip — kompletter Rider-Flow: preview → trip starten → in History prüfen
// Wird übersprungen wenn MAPBOX_TOKEN nicht gesetzt
func TestCreateTrip(t *testing.T) {
	if os.Getenv("MAPBOX_TOKEN") == "" {
		t.Skip("MAPBOX_TOKEN not set — skipping trip creation")
	}
	token := login(t, seedRider, seedPassword)
	userID := getUserID(t, token)

	// Schritt 1: Fare-Optionen holen
	previewResp := post(t, "/trip/preview", previewBody(userID), authHeader(token))
	defer previewResp.Body.Close()
	if previewResp.StatusCode != http.StatusOK {
		t.Fatalf("preview: want 200, got %d", previewResp.StatusCode)
	}
	var preview struct {
		Data struct {
			RideFares []map[string]any `json:"rideFares"`
		} `json:"data"`
	}
	if err := json.NewDecoder(previewResp.Body).Decode(&preview); err != nil {
		t.Fatalf("preview decode: %v", err)
	}
	if len(preview.Data.RideFares) == 0 {
		t.Fatal("preview: no fares returned")
	}
	fareID, ok := preview.Data.RideFares[0]["id"].(string)
	if !ok || fareID == "" {
		t.Fatalf("preview: no fare id in first option: %v", preview.Data.RideFares[0])
	}
	t.Logf("✓ preview ok, fareID=%s (%d options)", fareID, len(preview.Data.RideFares))

	// Schritt 2: Trip mit der ersten Fare starten
	startBody := fmt.Sprintf(`{"rideFareID":%q,"userID":%q}`, fareID, userID)
	startResp := post(t, "/trip/start", startBody, authHeader(token))
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusCreated {
		t.Fatalf("trip start: want 201, got %d", startResp.StatusCode)
	}
	var started struct {
		Data struct {
			TripID string `json:"tripID"`
		} `json:"data"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&started); err != nil {
		t.Fatalf("trip start decode: %v", err)
	}
	if started.Data.TripID == "" {
		t.Fatalf("trip start: no tripID in response")
	}
	t.Logf("✓ trip created id=%s", started.Data.TripID)

	// Schritt 3: Trip erscheint in der History
	histResp := get(t, "/trips/history", authHeader(token))
	defer histResp.Body.Close()
	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("history: want 200, got %d", histResp.StatusCode)
	}
	var trips []map[string]any
	if err := json.NewDecoder(histResp.Body).Decode(&trips); err != nil {
		t.Fatalf("history decode: %v", err)
	}
	found := false
	for _, tr := range trips {
		if tr["id"] == started.Data.TripID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("trip %s not found in history (got %d trips)", started.Data.TripID, len(trips))
	}
	t.Log("✓ trip appears in history")
}
*/
// ── ENDE MAPBOX-ABHÄNGIGE TESTS ───────────────────────────────────────────

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

// TestNearbyDrivers — Route existiert + Auth greift (hätte den Prod-404 gefangen)
func TestNearbyDrivers(t *testing.T) {
	resp := get(t, "/drivers/nearby?lat=51.22&lng=6.77&radius_km=15", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("nearby without token: want 401, got %d", resp.StatusCode)
	}

	token := login(t, seedRider, seedPassword)
	resp = get(t, "/drivers/nearby?lat=51.22&lng=6.77&radius_km=15", authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("nearby with token: want 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("nearby decode: %v", err)
	}
	if _, ok := out["drivers"]; !ok {
		t.Fatalf("nearby: missing 'drivers' key in response: %v", out)
	}
	t.Log("✓ /drivers/nearby ok (401 unauth, 200 + drivers-key auth)")
}

// TestTripRate — Endpoint verlangt Auth und akzeptiert ein Rating-Payload
// (war monatelang kaputt: Frontend schickte keinen Auth-Header → stilles 401)
func TestTripRate(t *testing.T) {
	resp := post(t, "/trip/rate", `{"trip_id":"00000000-0000-0000-0000-000000000000","rating":5}`, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("rate without token: want 401, got %d", resp.StatusCode)
	}

	token := login(t, seedRider, seedPassword)
	resp = post(t, "/trip/rate", `{"trip_id":"00000000-0000-0000-0000-000000000000","rating":5}`, authHeader(token))
	defer resp.Body.Close()
	// Nicht-existenter Trip: alles außer 401/404-Routing-Fehler ist ok —
	// wichtig ist, dass der authentifizierte Request den Handler erreicht.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		t.Fatalf("rate with token: handler not reached, got %d", resp.StatusCode)
	}
	t.Logf("✓ /trip/rate reachable with auth (status %d)", resp.StatusCode)
}

// TestLogout — Token ist nach Logout wirklich tot (Redis-Blacklist greift)
func TestLogout(t *testing.T) {
	token := login(t, seedRider, seedPassword)

	resp := post(t, "/v1/auth/logout", "", authHeader(token))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: want 200, got %d", resp.StatusCode)
	}

	resp = get(t, "/v1/users/me", authHeader(token))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token after logout: want 401, got %d — blacklist not working", resp.StatusCode)
	}
	t.Log("✓ logout ok, token revoked via blacklist")
}
