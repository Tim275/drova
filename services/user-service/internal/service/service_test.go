package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"drova/services/user-service/internal/domain"
	"go.uber.org/zap"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type stubStore struct {
	user        *domain.User
	createErr   error
	activateErr error
}

func (s *stubStore) Create(ctx context.Context, u *domain.User) error {
	if s.createErr != nil {
		return s.createErr
	}
	u.ID = 1
	s.user = u
	return nil
}
func (s *stubStore) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return s.user, nil
}
func (s *stubStore) GetByID(_ context.Context, _ int64) (*domain.User, error) {
	return s.user, nil
}
func (s *stubStore) Activate(_ context.Context, _ string) (*domain.User, error) {
	if s.activateErr != nil {
		return nil, s.activateErr
	}
	return s.user, nil
}
func (s *stubStore) UpdateProfile(_ context.Context, _ int64, displayName, avatarURL, phone, address string) error {
	s.user.DisplayName = displayName
	s.user.AvatarURL = avatarURL
	s.user.Phone = phone
	s.user.Address = address
	return nil
}

type stubInvitations struct{ deleted string }

func (s *stubInvitations) Create(_ context.Context, _ string, _ int64, _ time.Duration) error {
	return nil
}
func (s *stubInvitations) Delete(_ context.Context, token string) error {
	s.deleted = token
	return nil
}

type stubCache struct {
	stored *domain.User
	hit    bool
}

func (s *stubCache) Get(_ context.Context, _ int64) (*domain.User, error) {
	if s.hit {
		return s.stored, nil
	}
	return nil, errors.New("miss")
}
func (s *stubCache) Set(_ context.Context, u *domain.User) error {
	s.stored = u
	return nil
}
func (s *stubCache) Delete(_ context.Context, _ int64) { s.stored = nil }

type stubMailer struct{ called bool }

func (s *stubMailer) SendActivation(_, _ string) error {
	s.called = true
	return nil
}

// ── tests ────────────────────────────────────────────────────────────────────

func newSvc(store *stubStore, inv *stubInvitations, cache *stubCache, mailer *stubMailer) *UserService {
	return New(store, inv, cache, mailer, zap.NewNop().Sugar())
}

func TestRegister_HappyPath(t *testing.T) {
	svc := newSvc(&stubStore{}, &stubInvitations{}, &stubCache{}, &stubMailer{})

	u, err := svc.Register(context.Background(), "rider@test.com", "Test1234!", domain.RoleRider, "Tim", "+49123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 1 {
		t.Errorf("want ID=1, got %d", u.ID)
	}
	if u.Email != "rider@test.com" {
		t.Errorf("want email rider@test.com, got %s", u.Email)
	}
}

func TestRegister_StoreError(t *testing.T) {
	svc := newSvc(&stubStore{createErr: errors.New("duplicate email")}, &stubInvitations{}, &stubCache{}, &stubMailer{})

	_, err := svc.Register(context.Background(), "x@x.com", "pass", domain.RoleRider, "X", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRegister_PasswordIsHashed(t *testing.T) {
	store := &stubStore{}
	svc := newSvc(store, &stubInvitations{}, &stubCache{}, &stubMailer{})

	_, _ = svc.Register(context.Background(), "a@b.com", "plaintext", domain.RoleRider, "A", "")

	if store.user == nil {
		t.Fatal("user not stored")
	}
	if string(store.user.Password.Hash()) == "plaintext" {
		t.Error("password stored as plaintext")
	}
	if len(store.user.Password.Hash()) == 0 {
		t.Error("password hash is empty")
	}
}

func TestActivate_HappyPath(t *testing.T) {
	store := &stubStore{user: &domain.User{ID: 42, Email: "a@b.com"}}
	inv := &stubInvitations{}
	cache := &stubCache{stored: store.user, hit: true}
	svc := newSvc(store, inv, cache, &stubMailer{})

	u, err := svc.Activate(context.Background(), "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 42 {
		t.Errorf("want ID=42, got %d", u.ID)
	}
	if inv.deleted != "tok123" {
		t.Error("invitation not deleted")
	}
	if cache.stored != nil {
		t.Error("cache not cleared after activation")
	}
}

func TestActivate_StoreError(t *testing.T) {
	store := &stubStore{user: &domain.User{ID: 1}, activateErr: errors.New("token not found")}
	svc := newSvc(store, &stubInvitations{}, &stubCache{}, &stubMailer{})

	_, err := svc.Activate(context.Background(), "badtoken")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetByID_CacheHit(t *testing.T) {
	cached := &domain.User{ID: 7, Email: "cached@test.com"}
	cache := &stubCache{stored: cached, hit: true}
	store := &stubStore{user: &domain.User{ID: 99}} // should not be called
	svc := newSvc(store, &stubInvitations{}, cache, &stubMailer{})

	u, err := svc.GetByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 7 {
		t.Errorf("want ID=7 from cache, got %d", u.ID)
	}
}

func TestGetByID_CacheMiss_FallsBackToStore(t *testing.T) {
	dbUser := &domain.User{ID: 5, Email: "db@test.com"}
	cache := &stubCache{hit: false}
	store := &stubStore{user: dbUser}
	svc := newSvc(store, &stubInvitations{}, cache, &stubMailer{})

	u, err := svc.GetByID(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != 5 {
		t.Errorf("want ID=5, got %d", u.ID)
	}
	if cache.stored == nil {
		t.Error("user not written to cache after DB fetch")
	}
}

func TestUpdateProfile_ClearsCache(t *testing.T) {
	user := &domain.User{ID: 3}
	store := &stubStore{user: user}
	cache := &stubCache{stored: user, hit: true}
	svc := newSvc(store, &stubInvitations{}, cache, &stubMailer{})

	err := svc.UpdateProfile(context.Background(), 3, "New Name", "", "+49", "Berlin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache.stored != nil {
		t.Error("cache not cleared after profile update")
	}
	if store.user.DisplayName != "New Name" {
		t.Errorf("display name not updated, got %q", store.user.DisplayName)
	}
}

func TestGetByEmail_DelegatesToStore(t *testing.T) {
	u := &domain.User{ID: 9, Email: "mail@test.com"}
	store := &stubStore{user: u}
	svc := newSvc(store, &stubInvitations{}, &stubCache{}, &stubMailer{})

	got, err := svc.GetByEmail(context.Background(), "mail@test.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "mail@test.com" {
		t.Errorf("want email 'mail@test.com', got %q", got.Email)
	}
}

func TestGetByID_StoreError(t *testing.T) {
	cache := &stubCache{hit: false}
	store := &stubStore{} // user is nil → GetByID returns (nil, nil) or error
	svc := newSvc(store, &stubInvitations{}, cache, &stubMailer{})

	// GetByID with a nil user returned from store should still return without panic
	_, _ = svc.GetByID(context.Background(), 999)
}
