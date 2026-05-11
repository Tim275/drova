package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/mailer"
	"drova/services/user-service/internal/middleware"
	"drova/services/user-service/internal/service"
	"drova/services/user-service/internal/store"

	"go.uber.org/zap"
)

type fakeUserStore struct {
	users map[string]*domain.User
	byID  map[int64]*domain.User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		users: make(map[string]*domain.User),
		byID:  make(map[int64]*domain.User),
	}
}

func (f *fakeUserStore) Create(ctx context.Context, u *domain.User) error {
	if _, exists := f.users[u.Email]; exists {
		return fmt.Errorf("unique constraint")
	}
	u.ID = int64(len(f.users) + 1)
	u.CreatedAt = time.Now()
	f.users[u.Email] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserStore) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) GetByID(_ context.Context, id int64) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) Activate(_ context.Context, token string) (*domain.User, error) {
	for _, u := range f.byID {
		u.IsActivated = true
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUserStore) UpdateProfile(_ context.Context, userID int64, displayName, avatarURL, phone, address string) error {
	u, ok := f.byID[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.DisplayName = displayName
	u.AvatarURL = avatarURL
	u.Phone = phone
	u.Address = address
	return nil
}

type fakeInvStore struct{ tokens map[string]int64 }

func newFakeInvStore() *fakeInvStore { return &fakeInvStore{tokens: make(map[string]int64)} }
func (f *fakeInvStore) Create(_ context.Context, token string, uid int64, _ time.Duration) error {
	f.tokens[token] = uid
	return nil
}
func (f *fakeInvStore) Delete(_ context.Context, token string) error {
	delete(f.tokens, token)
	return nil
}

func newTestApp(t *testing.T) *application {
	app, _, _, _ := newTestAppFull(t)
	return app
}

func newTestAppFull(t *testing.T) (*application, *fakeUserStore, *fakeRefreshStore, *fakeBlacklist) {
	t.Helper()
	log, _ := zap.NewDevelopment()

	userStore := newFakeUserStore()
	invStore := newFakeInvStore()
	cache := store.NewUserCache(nil)
	authenticator := auth.NewAuthenticator("testsecret123456789012345678901", "drova", "drova-users")
	m := mailer.New("", "", "", "", "", "")
	svc := service.New(userStore, invStore, cache, m)
	bl := newFakeBlacklist()
	rt := newFakeRefreshStore()

	return &application{
		service:       svc,
		auth:          authenticator,
		blacklist:     bl,
		refreshTokens: rt,
		log:           log.Sugar(),
	}, userStore, rt, bl
}

func TestHandleRegister_Success(t *testing.T) {
	app := newTestApp(t)

	body := `{"display_name":"Test User","email":"test@drova.de","phone":"+4915112345678","password":"Secret123","role":"rider"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["email"] != "test@drova.de" {
		t.Errorf("want email test@drova.de, got %v", resp["email"])
	}
}

func TestHandleRegister_InvalidPayload(t *testing.T) {
	app := newTestApp(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad json", `{invalid}`, http.StatusBadRequest},
		{"missing email", `{"password":"secret123","role":"rider"}`, http.StatusUnprocessableEntity},
		{"short password", `{"email":"a@b.de","password":"short","role":"rider"}`, http.StatusUnprocessableEntity},
		{"bad role", `{"email":"a@b.de","password":"secret123","role":"admin"}`, http.StatusUnprocessableEntity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/users/register", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			app.handleRegister(w, req)
			if w.Code != tc.want {
				t.Errorf("want %d, got %d — %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	app.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// --- fakeBlacklist ---

type fakeBlacklist struct{ revoked map[string]bool }

func newFakeBlacklist() *fakeBlacklist { return &fakeBlacklist{revoked: make(map[string]bool)} }
func (f *fakeBlacklist) Revoke(_ context.Context, jti string, _ time.Duration) error {
	f.revoked[jti] = true
	return nil
}
func (f *fakeBlacklist) IsRevoked(_ context.Context, jti string) (bool, error) {
	return f.revoked[jti], nil
}

// --- fakeRefreshStore ---

type refreshEntry struct {
	userID int64
	role   string
}

type fakeRefreshStore struct{ tokens map[string]refreshEntry }

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{tokens: make(map[string]refreshEntry)}
}
func (f *fakeRefreshStore) Create(_ context.Context, token string, userID int64, role string, _ time.Duration) error {
	f.tokens[token] = refreshEntry{userID: userID, role: role}
	return nil
}
func (f *fakeRefreshStore) Validate(_ context.Context, token string) (int64, string, error) {
	e, ok := f.tokens[token]
	if !ok {
		return 0, "", errors.New("invalid token")
	}
	return e.userID, e.role, nil
}
func (f *fakeRefreshStore) Delete(_ context.Context, token string) error {
	delete(f.tokens, token)
	return nil
}

// --- helpers ---

func seedActivatedUser(t *testing.T, us *fakeUserStore, email, password string) *domain.User {
	t.Helper()
	u := &domain.User{
		Email:       email,
		Role:        domain.RoleRider,
		DisplayName: "Test Rider",
		Phone:       "+4915112345678",
		IsActivated: true,
	}
	if err := u.Password.Set(password); err != nil {
		t.Fatal(err)
	}
	u.ID = int64(len(us.byID) + 1)
	us.users[email] = u
	us.byID[u.ID] = u
	return u
}

func basicAuthHeader(email, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+password))
}

// --- Login (handleCreateToken) ---

func TestHandleCreateToken_Success(t *testing.T) {
	app, us, _, _ := newTestAppFull(t)
	seedActivatedUser(t, us, "login@drova.de", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", nil)
	req.Header.Set("Authorization", basicAuthHeader("login@drova.de", "Secret123!"))
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected non-empty access token in response")
	}
}

func TestHandleCreateToken_WrongPassword(t *testing.T) {
	app, us, _, _ := newTestAppFull(t)
	seedActivatedUser(t, us, "wp@drova.de", "Secret123!")

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", nil)
	req.Header.Set("Authorization", basicAuthHeader("wp@drova.de", "WrongPass!"))
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleCreateToken_NotActivated(t *testing.T) {
	app, us, _, _ := newTestAppFull(t)
	u := &domain.User{Email: "inactive@drova.de", Role: domain.RoleRider, IsActivated: false}
	if err := u.Password.Set("Secret123!"); err != nil {
		t.Fatal(err)
	}
	u.ID = 1
	us.users[u.Email] = u
	us.byID[u.ID] = u

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", nil)
	req.Header.Set("Authorization", basicAuthHeader("inactive@drova.de", "Secret123!"))
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleCreateToken_MissingAuth(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", nil)
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// --- GetMe (handleGetMe via Authenticate middleware) ---

func TestHandleGetMe_ValidToken(t *testing.T) {
	app, us, _, bl := newTestAppFull(t)
	u := seedActivatedUser(t, us, "getme@drova.de", "Secret123!")

	token, err := app.auth.GenerateToken(u.ID, string(u.Role))
	if err != nil {
		t.Fatal(err)
	}

	handler := middleware.Authenticate(app.auth, bl)(http.HandlerFunc(app.handleGetMe))
	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["email"] != "getme@drova.de" {
		t.Errorf("want email getme@drova.de, got %v", resp["email"])
	}
}

func TestHandleGetMe_Unauthorized(t *testing.T) {
	app, _, _, bl := newTestAppFull(t)
	handler := middleware.Authenticate(app.auth, bl)(http.HandlerFunc(app.handleGetMe))

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// --- Logout (handleLogout via Authenticate middleware) ---

func TestHandleLogout_RevokesToken(t *testing.T) {
	app, us, _, bl := newTestAppFull(t)
	u := seedActivatedUser(t, us, "logout@drova.de", "Secret123!")

	token, _ := app.auth.GenerateToken(u.ID, string(u.Role))
	handler := middleware.Authenticate(app.auth, bl)(http.HandlerFunc(app.handleLogout))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	// A second request with the same token must be rejected.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	middleware.Authenticate(app.auth, bl)(http.HandlerFunc(app.handleGetMe)).ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("revoked token must be rejected: want 401, got %d", w2.Code)
	}
}

// --- RefreshToken (handleRefreshToken) ---

func TestHandleRefreshToken_MissingCookie(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	w := httptest.NewRecorder()
	app.handleRefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleRefreshToken_InvalidToken(t *testing.T) {
	app := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-uuid-token"})
	w := httptest.NewRecorder()
	app.handleRefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandleRefreshToken_Success(t *testing.T) {
	app, _, rt, _ := newTestAppFull(t)
	_ = rt.Create(context.Background(), "valid-refresh-token", 42, "rider", time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "valid-refresh-token"})
	w := httptest.NewRecorder()
	app.handleRefreshToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected non-empty access token in response")
	}
}
