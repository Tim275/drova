package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("already exists")
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrInvalidPassword = errors.New("invalid password")
	ErrNotActivated    = errors.New("account not activated")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
)
