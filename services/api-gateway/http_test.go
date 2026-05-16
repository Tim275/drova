package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── handleHealth ──────────────────────────────────────────────────────────────

func TestHandleHealth_Returns200(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want application/json, got %q", ct)
	}
}

func TestHandleHealth_BodyContainsAlive(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, r)

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["data"] != "i am alive" {
		t.Errorf("want data='i am alive', got %v", resp["data"])
	}
}

// ── handleConfig ──────────────────────────────────────────────────────────────

func TestHandleConfig_Returns200WithKeys(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	handleConfig(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := resp["mapboxToken"]; !ok {
		t.Error("expected 'mapboxToken' key in response")
	}
	if _, ok := resp["stripePublishableKey"]; !ok {
		t.Error("expected 'stripePublishableKey' key in response")
	}
}

// ── handleReadyz ──────────────────────────────────────────────────────────────

type okKafka struct{}

func (okKafka) Ping(_ context.Context) error { return nil }

type failKafka struct{}

func (failKafka) Ping(_ context.Context) error { return errors.New("kafka down") }

func TestHandleReadyz_KafkaOK(t *testing.T) {
	handler := handleReadyz(okKafka{})
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandleReadyz_KafkaDown_Returns503(t *testing.T) {
	handler := handleReadyz(failKafka{})
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// ── handleTripHistory ─────────────────────────────────────────────────────────

func TestHandleTripHistory_NoToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/trips/history", nil)
	w := httptest.NewRecorder()
	handleTripHistory(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleTripHistory_InvalidToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/trips/history", nil)
	r.Header.Set("Authorization", "Bearer not.a.real.token")
	w := httptest.NewRecorder()
	handleTripHistory(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── handleDriverHistory ───────────────────────────────────────────────────────

func TestHandleDriverHistory_NoAuth_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/trips/driver-history", nil)
	w := httptest.NewRecorder()
	handleDriverHistory(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── handleTripStart ───────────────────────────────────────────────────────────

func TestHandleTripStart_BadJSON_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/trip/start", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	handleTripStart(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleTripStart_EmptyBody_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/trip/start", strings.NewReader(""))
	w := httptest.NewRecorder()
	handleTripStart(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── handleTripPreview ─────────────────────────────────────────────────────────

func TestHandleTripPreview_BadJSON_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/trip/preview", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	handleTripPreview(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleTripPreview_EmptyBody_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/trip/preview", strings.NewReader(""))
	w := httptest.NewRecorder()
	handleTripPreview(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── handleTripRate ────────────────────────────────────────────────────────────

func TestHandleTripRate_BadJSON_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/trip/rate", strings.NewReader("bad"))
	w := httptest.NewRecorder()
	handleTripRate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── handleRegister ────────────────────────────────────────────────────────────

func TestHandleRegister_BadJSON_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/users/register", strings.NewReader("{bad json"))
	w := httptest.NewRecorder()
	handleRegister(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandleRegister_EmptyBody_Returns400(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/users/register", strings.NewReader(""))
	w := httptest.NewRecorder()
	handleRegister(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── handleGetMe ───────────────────────────────────────────────────────────────

func TestHandleGetMe_NoToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	w := httptest.NewRecorder()
	handleGetMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleGetMe_InvalidToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	r.Header.Set("Authorization", "Bearer garbage.token.here")
	w := httptest.NewRecorder()
	handleGetMe(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── handleUpdateProfile ───────────────────────────────────────────────────────

func TestHandleUpdateProfile_NoToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/v1/users/profile", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	handleUpdateProfile(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleUpdateProfile_InvalidToken_Returns401(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/v1/users/profile", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer not.valid.token")
	w := httptest.NewRecorder()
	handleUpdateProfile(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleUpdateProfile_ValidToken_BadJSON_Returns400(t *testing.T) {
	jwtSecret = testSecret
	claims := &gatewayClaims{}
	claims.UserID = 42
	ctx := context.WithValue(context.Background(), claimsKey, claims)

	r := httptest.NewRequest(http.MethodPut, "/v1/users/profile", strings.NewReader("{bad"))
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	handleUpdateProfile(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
