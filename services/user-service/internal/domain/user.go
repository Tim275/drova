package domain

import (
	"context"
	"time"
)

type Role string

const (
	RoleRider  Role = "rider"
	RoleDriver Role = "driver"
)

type User struct {
	ID          int64
	Email       string
	Password    password
	Role        Role
	IsActivated bool
	CreatedAt   time.Time
	DisplayName string
	AvatarURL   string
	Phone       string
	Address     string
}

type password struct {
	hash []byte
}

func (p *password) Set(plain string) error {
	hash, err := hashPassword(plain)
	if err != nil {
		return err
	}
	p.hash = hash
	return nil
}

func (p *password) Matches(plain string) (bool, error) {
	return comparePassword(p.hash, plain)
}

func (p *password) Hash() []byte    { return p.hash }
func (p *password) SetHash(h []byte) { p.hash = h }

type UserStore interface {
	Create(ctx context.Context, user *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	Activate(ctx context.Context, token string) (*User, error)
	UpdateProfile(ctx context.Context, userID int64, displayName, avatarURL, phone, address string) error
}

type InvitationStore interface {
	Create(ctx context.Context, token string, userID int64, expiry time.Duration) error
	Delete(ctx context.Context, token string) error
}

type UserCacher interface {
	Get(ctx context.Context, id int64) (*User, error)
	Set(ctx context.Context, u *User) error
	Delete(ctx context.Context, id int64)
}

type EmailSender interface {
	SendActivation(toEmail, token string) error
}

type TokenBlacklist interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
