package middleware

import (
	"context"
	"net/http"
	"strings"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
)

type contextKey string

const UserClaimsKey contextKey = "userClaims"

func Authenticate(authenticator *auth.Authenticator, bl domain.TokenBlacklist) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				WriteError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := authenticator.ValidateToken(tokenStr)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			revoked, err := bl.IsRevoked(r.Context(), claims.ID)
			if err == nil && revoked {
				WriteError(w, http.StatusUnauthorized, "token revoked")
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetClaims(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(UserClaimsKey).(*auth.Claims)
	return claims
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserClaimsKey).(*auth.Claims)
			if !ok || claims.Role != role {
				WriteError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
