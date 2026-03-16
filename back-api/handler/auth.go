package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/jackc/pgx/v5"
)

const maxBodySize = 1 << 20 // 1 MB

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req registerRequest

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username, email, and password are required")
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email format")
		return
	}

	if len(req.Password) < 10 {
		writeError(w, http.StatusBadRequest, "password must be at least 10 characters")
		return
	}

	// bcrypt silently truncates input beyond 72 bytes — two passwords
	// that differ only after byte 72 would hash identically.
	if len(req.Password) > 72 {
		writeError(w, http.StatusBadRequest, "password must not exceed 72 bytes")
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	if err := h.Repo.RunTx(r.Context(), func(tx pgx.Tx) error {
		txRepo := h.Repo.WithTx(tx)
		if err := txRepo.CreateUser(r.Context(), user); err != nil {
			return err
		}
		return txRepo.AssignRoleToUser(r.Context(), user.ID, auth.RoleUser)
	}); err != nil {
		if apperr.IsConflict(err) {
			writeError(w, http.StatusConflict, "username or email already exists")
			return
		}
		slog.Error("failed to create user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		Message: "user registered successfully",
		UserID:  user.ID,
	})
}

// Login handles user authentication
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req loginRequest

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Brute-force protection: max 10 failed attempts per 15 minutes.
	// Atomically increment first to avoid TOCTOU race between GET and INCR.
	const maxAttempts int64 = 10
	const lockoutWindow = 15 * time.Minute

	attempts, err := h.Sessions.IncrLoginAttempts(r.Context(), req.Email, lockoutWindow)
	if err != nil {
		slog.Warn("failed to check login attempts, allowing request", "email", req.Email, "error", err)
	} else if attempts > maxAttempts {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	user, err := h.Repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if apperr.IsNotFound(err) {
			slog.Warn("failed login attempt", "email", req.Email, "reason", "user not found")
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		slog.Error("failed to get user", "email", req.Email, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		slog.Warn("failed login attempt", "email", req.Email, "reason", "wrong password")
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Successful login — reset counter so it doesn't penalize the user.
	if err := h.Sessions.ResetLoginAttempts(r.Context(), req.Email); err != nil {
		slog.Warn("failed to reset login attempts", "email", req.Email, "error", err)
	}

	tokens, err := h.issueTokenPair(r.Context(), user)
	if err != nil {
		slog.Error("failed to issue tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// Refresh handles token refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req tokenRequest

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	// Rate-limit refresh attempts using the token hash as key.
	// Atomically increment first to avoid TOCTOU race.
	tokenHash := auth.HashToken(req.RefreshToken)

	const maxRefreshAttempts int64 = 5
	const refreshWindow = 5 * time.Minute
	refreshKey := "refresh:" + tokenHash
	attempts, err := h.Sessions.IncrLoginAttempts(r.Context(), refreshKey, refreshWindow)
	if err != nil {
		slog.Warn("failed to check refresh attempts, allowing request", "error", err)
	} else if attempts > maxRefreshAttempts {
		writeError(w, http.StatusTooManyRequests, "too many refresh attempts, try again later")
		return
	}

	userID, err := h.Sessions.GetRefreshTokenUserID(r.Context(), tokenHash)
	if err != nil {
		slog.Warn("refresh token lookup failed", "error", err)
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		slog.Error("invalid user ID from refresh token", "user_id", userID)
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	user, err := h.Repo.GetUserByID(r.Context(), userUUID)
	if err != nil {
		slog.Error("failed to get user", "error", err)
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	// Revoke old refresh token before issuing new pair
	if err := h.Sessions.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		slog.Error("failed to revoke old refresh token", "error", err)
	}

	tokens, err := h.issueTokenPair(r.Context(), user)
	if err != nil {
		slog.Error("failed to issue tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// issueTokenPair creates a JWT access/refresh token pair, stores the refresh token
// in Redis and records it in the database. Shared by Login and Refresh handlers.
func (h *AuthHandler) issueTokenPair(ctx context.Context, user *models.User) (*auth.TokenPair, error) {
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		h.Config.JWT.Secret,
		h.Config.JWT.Issuer,
		user.ID,
		roleNames,
		h.Config.JWT.TTLAccess,
		h.Config.JWT.TTLRefresh,
	)
	if err != nil {
		return nil, fmt.Errorf("create token pair: %w", err)
	}

	tokenHash := auth.HashToken(tokens.RefreshToken)

	if err := h.Sessions.StoreRefreshToken(ctx, user.ID.String(), tokenHash, h.Config.JWT.TTLRefresh); err != nil {
		return nil, fmt.Errorf("store refresh token in Redis: %w", err)
	}

	if err := h.Repo.CreateRefreshToken(ctx, &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(h.Config.JWT.TTLRefresh),
	}); err != nil {
		// DB record is secondary — log but don't fail the login
		slog.Error("failed to persist refresh token to DB", "error", err)
	}

	return tokens, nil
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req tokenRequest

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken != "" {
		tokenHash := auth.HashToken(req.RefreshToken)
		if err := h.Sessions.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token", "error", err)
		}
		if err := h.Repo.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token in db", "error", err)
		}
	}

	// Blacklist the current access token so it can't be reused after logout
	if claims, ok := auth.FromContext(r.Context()); ok {
		raw := r.Header.Get("Authorization")
		if tokenStr := strings.TrimPrefix(raw, "Bearer "); tokenStr != raw && tokenStr != "" {
			tokenHash := auth.HashToken(tokenStr)
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				if err := h.Sessions.StoreBlacklistedToken(r.Context(), tokenHash, ttl); err != nil {
					slog.Error("failed to blacklist access token", "error", err)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}
