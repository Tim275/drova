package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drova/services/user-service/internal/auth"
)

// --- fakes ---

type noopBlacklist struct{}

func (noopBlacklist) Revoke(_ context.Context, _ string, _ time.Duration) error { return nil }
func (noopBlacklist) IsRevoked(_ context.Context, _ string) (bool, error)       { return false, nil }

type alwaysRevokedBlacklist struct{}

func (alwaysRevokedBlacklist) Revoke(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (alwaysRevokedBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func newTestAuthenticator() *auth.Authenticator {
	return auth.NewAuthenticator("testsecret123456789012345678901", "drova", "drova-users")
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// --- Authenticate ---

func TestAuthenticate_MissingToken(t *testing.T) {
	a := newTestAuthenticator()
	handler := Authenticate(a, noopBlacklist{})(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	a := newTestAuthenticator()
	handler := Authenticate(a, noopBlacklist{})(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAuthenticate_RevokedToken(t *testing.T) {
	a := newTestAuthenticator()
	token, err := a.GenerateToken(1, "rider")
	if err != nil {
		t.Fatal(err)
	}

	handler := Authenticate(a, alwaysRevokedBlacklist{})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for revoked token, got %d", w.Code)
	}
}

func TestAuthenticate_ValidToken_PassesThrough(t *testing.T) {
	a := newTestAuthenticator()
	token, err := a.GenerateToken(42, "rider")
	if err != nil {
		t.Fatal(err)
	}

	var captured *auth.Claims
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetClaims(r)
		w.WriteHeader(http.StatusOK)
	})

	handler := Authenticate(a, noopBlacklist{})(captureHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if captured == nil {
		t.Fatal("claims must be injected into request context")
	}
	if captured.UserID != 42 {
		t.Errorf("want userID 42, got %d", captured.UserID)
	}
	if captured.Role != "rider" {
		t.Errorf("want role rider, got %s", captured.Role)
	}
}

// --- RequireRole ---

func TestRequireRole_WrongRole(t *testing.T) {
	a := newTestAuthenticator()
	token, _ := a.GenerateToken(1, "rider")

	handler := Authenticate(a, noopBlacklist{})(RequireRole("driver")(http.HandlerFunc(okHandler)))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403 for wrong role, got %d", w.Code)
	}
}

func TestRequireRole_CorrectRole(t *testing.T) {
	a := newTestAuthenticator()
	token, _ := a.GenerateToken(1, "driver")

	handler := Authenticate(a, noopBlacklist{})(RequireRole("driver")(http.HandlerFunc(okHandler)))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200 for correct role, got %d", w.Code)
	}
}
