package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testSecret = "testsecret123456789012345678901"
	testIssuer = "drova"
	testAud    = "drova-users"
)

func newTestAuth() *Authenticator {
	return NewAuthenticator(testSecret, testIssuer, testAud)
}

func TestGenerateToken_ValidToken(t *testing.T) {
	a := newTestAuth()
	token, err := a.GenerateToken(42, "rider")
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}
	if token == "" {
		t.Fatal("want non-empty token")
	}
	if len(strings.Split(token, ".")) != 3 {
		t.Errorf("want 3-part JWT, got: %s", token)
	}
}

func TestGenerateToken_UniqueJTI(t *testing.T) {
	a := newTestAuth()
	t1, _ := a.GenerateToken(1, "rider")
	t2, _ := a.GenerateToken(1, "rider")

	c1, _ := a.ValidateToken(t1)
	c2, _ := a.ValidateToken(t2)

	if c1.ID == c2.ID {
		t.Error("each token must have a unique JTI")
	}
}

func TestValidateToken_RoundTrip(t *testing.T) {
	a := newTestAuth()
	tests := []struct {
		userID int64
		role   string
	}{
		{1, "rider"},
		{99, "driver"},
	}

	for _, tc := range tests {
		token, err := a.GenerateToken(tc.userID, tc.role)
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		claims, err := a.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken: %v", err)
		}
		if claims.UserID != tc.userID {
			t.Errorf("want userID %d, got %d", tc.userID, claims.UserID)
		}
		if claims.Role != tc.role {
			t.Errorf("want role %q, got %q", tc.role, claims.Role)
		}
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	signer := NewAuthenticator("correct-secret-32bytes-longXXXXX", testIssuer, testAud)
	validator := NewAuthenticator("wrong-secret-32bytes-longXXXXXX!", testIssuer, testAud)

	token, _ := signer.GenerateToken(1, "rider")
	_, err := validator.ValidateToken(token)
	if err == nil {
		t.Error("want error for wrong secret, got nil")
	}
}

func TestValidateToken_WrongAudience(t *testing.T) {
	a := NewAuthenticator(testSecret, testIssuer, "other-service")
	token, _ := a.GenerateToken(1, "rider")

	validator := newTestAuth() // expects "drova-users"
	_, err := validator.ValidateToken(token)
	if err == nil {
		t.Error("want error for wrong audience, got nil")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	a := newTestAuth()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAud},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		UserID: 1,
		Role:   "rider",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))

	_, err := a.ValidateToken(signed)
	if err == nil {
		t.Error("want error for expired token, got nil")
	}
}

func TestValidateToken_AlgorithmPinning(t *testing.T) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAud},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: 1,
		Role:   "rider",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
	signed, _ := token.SignedString([]byte(testSecret))

	a := newTestAuth()
	_, err := a.ValidateToken(signed)
	if err == nil {
		t.Error("want error for disallowed algorithm HS384, got nil")
	}
}
