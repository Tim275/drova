package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/mailer"
	"drova/services/user-service/internal/service"
	"drova/services/user-service/internal/store"

	"go.uber.org/zap"
)

// --- Fakes ---

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

type fakeMailer struct{ sent []string }

func (m *fakeMailer) SendActivation(email, token string) error {
	m.sent = append(m.sent, email)
	return nil
}

// --- Helpers ---

func newTestApp(t *testing.T) *application {
	t.Helper()
	log, _ := zap.NewDevelopment()

	userStore := newFakeUserStore()
	invStore := newFakeInvStore()
	cache := store.NewUserCache(nil) // nil rdb — cache ops are no-ops with nil client

	fm := &fakeMailer{}
	_ = fm

	authenticator := auth.NewAuthenticator("testsecret123456789012345678901", "drova", "drova-users")

	// We wire a real mailer stub that doesn't send emails
	m := mailer.New("", "", "", "")
	svc := service.New(userStore, invStore, cache, m)

	return &application{
		service: svc,
		auth:    authenticator,
		log:     log.Sugar(),
	}
}

// --- Tests ---

func TestHandleRegister_Success(t *testing.T) {
	app := newTestApp(t)

	body := `{"email":"test@drova.de","password":"secret123","role":"rider"}`
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
