package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"drova/shared/env"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(env.GetString("JWT_SECRET", ""))

type ctxKey int

const claimsKey ctxKey = 0

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

func claimsFromContext(ctx context.Context) *gatewayClaims {
	c, _ := ctx.Value(claimsKey).(*gatewayClaims)
	return c
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
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// resolveWSAuth authenticates a WebSocket upgrade request. It first tries a
// short-lived one-time ticket from Redis (issued by GET /ws/ticket), then falls
// back to a Bearer JWT in the ?token= query param for dev/legacy clients.
func resolveWSAuth(r *http.Request) (*gatewayClaims, error) {
	if ticket := r.URL.Query().Get("ticket"); ticket != "" {
		if gatewayRdb == nil {
			return nil, errors.New("ticket auth unavailable")
		}
		val, err := gatewayRdb.GetDel(r.Context(), "wstkt:"+ticket).Result()
		if err != nil {
			return nil, errors.New("invalid or expired ticket")
		}
		parts := strings.SplitN(val, "|", 3)
		if len(parts) != 3 {
			return nil, errors.New("malformed ticket")
		}
		uid, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, errors.New("malformed ticket uid")
		}
		claims := &gatewayClaims{
			RegisteredClaims: jwt.RegisteredClaims{ID: parts[2]},
			UserID:           uid,
			Role:             parts[1],
		}
		if isBlacklisted(r.Context(), claims.ID) {
			return nil, errors.New("token revoked")
		}
		return claims, nil
	}
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		return nil, errors.New("no auth provided")
	}
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if isBlacklisted(r.Context(), claims.ID) {
		return nil, errors.New("token revoked")
	}
	return claims, nil
}
