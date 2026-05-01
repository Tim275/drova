package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"drova/services/user-service/internal/domain"
)

const invitationExpiry = 72 * time.Hour

type UserService struct {
	users       domain.UserStore
	invitations domain.InvitationStore
	cache       domain.UserCacher
	mailer      domain.EmailSender
}

func New(users domain.UserStore, invitations domain.InvitationStore, cache domain.UserCacher, mailer domain.EmailSender) *UserService {
	return &UserService{users: users, invitations: invitations, cache: cache, mailer: mailer}
}

func (s *UserService) Register(ctx context.Context, email, plainPassword string, role domain.Role, displayName, phone string) (*domain.User, error) {
	user := &domain.User{Email: email, Role: role, DisplayName: displayName, Phone: phone}
	if err := user.Password.Set(plainPassword); err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	if err := s.invitations.Create(ctx, token, user.ID, invitationExpiry); err != nil {
		return nil, err
	}

	go func() { _ = s.mailer.SendActivation(user.Email, token) }()
	return user, nil
}

func (s *UserService) Activate(ctx context.Context, token string) (*domain.User, error) {
	user, err := s.users.Activate(ctx, token)
	if err != nil {
		return nil, err
	}
	_ = s.invitations.Delete(ctx, token)
	s.cache.Delete(ctx, user.ID)
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	if u, err := s.cache.Get(ctx, id); err == nil && u != nil {
		return u, nil
	}

	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	_ = s.cache.Set(ctx, u)
	return u, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.users.GetByEmail(ctx, email)
}

func (s *UserService) UpdateProfile(ctx context.Context, userID int64, displayName, avatarURL, phone, address string) error {
	if err := s.users.UpdateProfile(ctx, userID, displayName, avatarURL, phone, address); err != nil {
		return err
	}
	s.cache.Delete(ctx, userID)
	return nil
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
