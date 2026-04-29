package domain

import (
	"golang.org/x/crypto/bcrypt"
)

func hashPassword(plain string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plain), 12)
}

func comparePassword(hash []byte, plain string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(hash, []byte(plain))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
