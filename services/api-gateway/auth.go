package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"drova/shared/env"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(env.GetString("JWT_SECRET", ""))

type gatewayClaims struct {
	jwt.RegisteredClaims
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
}

func parseGatewayToken(tokenStr string) (*gatewayClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &gatewayClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	},
		jwt.WithExpirationRequired(),
		jwt.WithAudience("drova-users"),
		jwt.WithIssuer("drova"),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*gatewayClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

func tokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func isBlacklisted(ctx context.Context, jti string) bool {
	if gatewayRdb == nil {
		return false
	}
	n, err := gatewayRdb.Exists(ctx, "jti:revoked:"+jti).Result()
	return err == nil && n > 0
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := tokenFromRequest(r)
		if tokenStr == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		claims, err := parseGatewayToken(tokenStr)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if isBlacklisted(r.Context(), claims.ID) {
			http.Error(w, "token revoked", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
