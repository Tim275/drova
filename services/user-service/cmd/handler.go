package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/middleware"
	"drova/shared/env"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const refreshTokenTTL = 7 * 24 * time.Hour
const refreshCookieName = "refresh_token"

func secureCookie() bool {
	return env.GetString("ENVIRONMENT", "development") != "development"
}

type registerRequest struct {
	DisplayName string      `json:"display_name" validate:"required,min=2,max=50"`
	Email       string      `json:"email"        validate:"required,email"`
	Phone       string      `json:"phone"        validate:"required,min=7,max=20,phone"`
	Password    string      `json:"password"     validate:"required,min=8,strongpassword"`
	Role        domain.Role `json:"role"         validate:"required,oneof=rider driver"`
}

func (app *application) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := middleware.ReadJSON(r, &req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !middleware.Validate(w, req) {
		return
	}

	user, err := app.service.Register(r.Context(), req.Email, req.Password, req.Role, req.DisplayName, req.Phone)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			middleware.WriteError(w, http.StatusConflict, "email already registered")
			return
		}
		app.log.Errorw("register", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":      user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"message": "check your email to activate your account",
	})
}

func (app *application) handleActivate(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		middleware.WriteError(w, http.StatusBadRequest, "token required")
		return
	}

	if _, err := app.service.Activate(r.Context(), token); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Token already used or expired — redirect gracefully so user lands on login
			http.Redirect(w, r, "/?activated=true", http.StatusSeeOther)
			return
		}
		app.log.Errorw("activate", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "activation failed")
		return
	}

	http.Redirect(w, r, "/?activated=true", http.StatusSeeOther)
}

func (app *application) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	email, password, ok := parseBasicAuth(r)
	if !ok {
		middleware.WriteError(w, http.StatusUnauthorized, "invalid basic auth")
		return
	}

	user, err := app.service.GetByEmail(r.Context(), email)
	if err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.IsActivated {
		middleware.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	match, err := user.Password.Matches(password)
	if err != nil || !match {
		middleware.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := app.auth.GenerateToken(user.ID, string(user.Role))
	if err != nil {
		app.log.Errorw("generate token", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	refreshToken := uuid.New().String()
	if err := app.refreshTokens.Create(r.Context(), refreshToken, user.ID, string(user.Role), refreshTokenTTL); err != nil {
		app.log.Warnw("create refresh token", zap.Error(err))
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:     refreshCookieName,
			Value:    refreshToken,
			HttpOnly: true,
			Secure:   secureCookie(),
			SameSite: http.SameSiteStrictMode,
			Path:     "/v1/auth/",
			MaxAge:   int(refreshTokenTTL.Seconds()),
		})
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (app *application) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		middleware.WriteError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	userID, role, err := app.refreshTokens.Validate(r.Context(), cookie.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: refreshCookieName, MaxAge: -1, Path: "/v1/auth/", Secure: secureCookie()})
		middleware.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Generate JWT first — if this fails, we don't touch the refresh token.
	token, err := app.auth.GenerateToken(userID, role)
	if err != nil {
		app.log.Errorw("generate token on refresh", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	// Rotate: issue new first, then delete old. If Create fails, the old token remains valid.
	newRefresh := uuid.New().String()
	if err := app.refreshTokens.Create(r.Context(), newRefresh, userID, role, refreshTokenTTL); err != nil {
		app.log.Warnw("rotate refresh token", zap.Error(err))
	} else {
		_ = app.refreshTokens.Delete(r.Context(), cookie.Value)
		http.SetCookie(w, &http.Cookie{
			Name:     refreshCookieName,
			Value:    newRefresh,
			HttpOnly: true,
			Secure:   secureCookie(),
			SameSite: http.SameSiteStrictMode,
			Path:     "/v1/auth/",
			MaxAge:   int(refreshTokenTTL.Seconds()),
		})
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (app *application) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	user, err := app.service.GetByID(r.Context(), claims.UserID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         user.Role,
		"is_activated": user.IsActivated,
		"created_at":   user.CreatedAt,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"phone":        user.Phone,
		"address":      user.Address,
	})
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name" validate:"omitempty,min=2,max=50"`
	AvatarURL   string `json:"avatar_url"   validate:"omitempty,max=500"`
	Phone       string `json:"phone"        validate:"omitempty,min=7,max=20,phone"`
	Address     string `json:"address"      validate:"omitempty,max=200"`
}

func (app *application) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	var req updateProfileRequest
	if err := middleware.ReadJSON(r, &req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !middleware.Validate(w, req) {
		return
	}
	if err := app.service.UpdateProfile(r.Context(), claims.UserID, req.DisplayName, req.AvatarURL, req.Phone, req.Address); err != nil {
		app.log.Errorw("update profile", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	user, _ := app.service.GetByID(r.Context(), claims.UserID)
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"display_name": user.DisplayName,
		"avatar_url":   user.AvatarURL,
		"phone":        user.Phone,
		"address":      user.Address,
	})
}

func (app *application) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims != nil && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			if err := app.blacklist.Revoke(r.Context(), claims.ID, ttl); err != nil {
				app.log.Warnw("revoke token", "jti", claims.ID, "err", err)
			}
		}
	}
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		_ = app.refreshTokens.Delete(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, MaxAge: -1, Path: "/v1/auth/", Secure: secureCookie()})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (app *application) handleHealth(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseBasicAuth(r *http.Request) (string, string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Basic ") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
