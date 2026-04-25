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
}

type InvitationStore interface {
	Create(ctx context.Context, token string, userID int64, expiry time.Duration) error
	Delete(ctx context.Context, token string) error
}

// UserCacher abstracts the caching layer — service layer never imports Redis directly.
type UserCacher interface {
	Get(ctx context.Context, id int64) (*User, error)
	Set(ctx context.Context, u *User) error
	Delete(ctx context.Context, id int64)
}

// EmailSender abstracts email delivery — service layer never imports mailer directly.
type EmailSender interface {
	SendActivation(toEmail, token string) error
}
