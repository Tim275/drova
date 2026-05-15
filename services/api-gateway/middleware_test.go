package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() {
	// grpcErrToHTTP logs via appLog on the default case — init a no-op logger.
	appLog = zap.NewNop().Sugar()
}

// ── requestID ─────────────────────────────────────────────────────────────────

func TestRequestID_GeneratesIDWhenMissing(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("expected X-Request-ID to be set")
	}
}

func TestRequestID_PassesThroughExistingID(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "fixed-id-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if got := w.Header().Get("X-Request-ID"); got != "fixed-id-123" {
		t.Errorf("want 'fixed-id-123', got %q", got)
	}
}

// ── enableCORS ────────────────────────────────────────────────────────────────

func TestEnableCORS_SetsHeaders(t *testing.T) {
	inner := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	handler := enableCORS(inner)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
}

func TestEnableCORS_OptionsReturns200(t *testing.T) {
	called := false
	inner := func(w http.ResponseWriter, r *http.Request) { called = true }
	handler := enableCORS(inner)

	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	handler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200 for OPTIONS, got %d", w.Code)
	}
	if called {
		t.Error("inner handler should not be called for OPTIONS preflight")
	}
}

// ── securityHeaders ────────────────────────────────────────────────────────────

func TestSecurityHeaders_Present(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	}
	for name, want := range headers {
		if got := w.Header().Get(name); got != want {
			t.Errorf("header %s: want %q, got %q", name, want, got)
		}
	}
	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy header missing")
	}
}

// ── handleOptions ──────────────────────────────────────────────────────────────

func TestHandleOptions_Returns200(t *testing.T) {
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	handleOptions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── writeJSON ─────────────────────────────────────────────────────────────────

func TestWriteJSON_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()
	_ = writeJSON(w, http.StatusOK, map[string]string{"key": "val"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want 'application/json', got %q", ct)
	}
}

func TestWriteJSON_WritesValidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	_ = writeJSON(w, http.StatusCreated, map[string]int{"count": 3})

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}
	var result map[string]int
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["count"] != 3 {
		t.Errorf("want count=3, got %d", result["count"])
	}
}

// ── grpcErrToHTTP ──────────────────────────────────────────────────────────────

func TestGrpcErrToHTTP_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	grpcErrToHTTP(w, status.Error(codes.NotFound, "trip not found"))
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGrpcErrToHTTP_AlreadyExists(t *testing.T) {
	w := httptest.NewRecorder()
	grpcErrToHTTP(w, status.Error(codes.AlreadyExists, "email taken"))
	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

func TestGrpcErrToHTTP_Unauthenticated(t *testing.T) {
	w := httptest.NewRecorder()
	grpcErrToHTTP(w, status.Error(codes.Unauthenticated, "bad creds"))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestGrpcErrToHTTP_InvalidArgument(t *testing.T) {
	w := httptest.NewRecorder()
	grpcErrToHTTP(w, status.Error(codes.InvalidArgument, "bad input"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGrpcErrToHTTP_UnknownFallsToInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	grpcErrToHTTP(w, status.Error(codes.Internal, "server crash"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

