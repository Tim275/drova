package main

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/middleware"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type registerRequest struct {
	Email    string      `json:"email"    validate:"required,email"`
	Password string      `json:"password" validate:"required,min=8"`
	Role     domain.Role `json:"role"     validate:"required,oneof=rider driver"`
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

	user, err := app.service.Register(r.Context(), req.Email, req.Password, req.Role)
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

	user, err := app.service.Activate(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			middleware.WriteError(w, http.StatusNotFound, "invalid or expired token")
			return
		}
		app.log.Errorw("activate", zap.Error(err))
		middleware.WriteError(w, http.StatusInternalServerError, "activation failed")
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"is_activated": user.IsActivated,
	})
}

// POST /v1/auth/token  — Basic Auth → JWT
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
		middleware.WriteError(w, http.StatusForbidden, "account not activated")
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
	})
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
