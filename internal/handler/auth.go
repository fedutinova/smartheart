package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"time"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/common"
	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
)

const maxBodySize = 1 << 20 // 1 MB

// Register handles user registration
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, and password are required", http.StatusBadRequest)
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	passwordHash, err := h.Repo.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
	}

	if err := h.Repo.DB().WithTx(r.Context(), func(tx database.Tx) error {
		txRepo := h.Repo.WithTx(tx)
		if err := txRepo.CreateUser(r.Context(), user); err != nil {
			return err
		}
		return txRepo.AssignRoleToUser(r.Context(), user.ID, auth.RoleUser)
	}); err != nil {
		if common.IsConflict(err) {
			http.Error(w, "username or email already exists", http.StatusConflict)
			return
		}
		slog.Error("failed to create user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.RegisterResponse{
		Message: "user registered successfully",
		UserID:  user.ID,
	})
}

// Login handles user authentication
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	// Brute-force protection: max 10 failed attempts per 15 minutes
	const maxAttempts int64 = 10
	const lockoutWindow = 15 * time.Minute

	attempts, err := h.Redis.GetLoginAttempts(r.Context(), req.Email)
	if err == nil && attempts >= maxAttempts {
		http.Error(w, "too many login attempts, try again later", http.StatusTooManyRequests)
		return
	}

	user, err := h.Repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if common.IsNotFound(err) {
			h.Redis.IncrLoginAttempts(r.Context(), req.Email, lockoutWindow)
			slog.Warn("login attempt with invalid email", "email", req.Email)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		slog.Error("failed to get user", "email", req.Email, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !h.Repo.CheckPassword(req.Password, user.PasswordHash) {
		h.Redis.IncrLoginAttempts(r.Context(), req.Email, lockoutWindow)
		slog.Warn("login attempt with invalid password", "email", req.Email)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	h.Redis.ResetLoginAttempts(r.Context(), req.Email)

	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		h.Config.JWTSecret,
		h.Config.JWTIssuer,
		user.ID,
		roleNames,
		h.Config.JWTTTLAccess,
		h.Config.JWTTTLRefresh,
	)
	if err != nil {
		slog.Error("failed to create token pair", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tokenHash := h.Repo.HashRefreshToken(tokens.RefreshToken)

	if err := h.Redis.StoreRefreshToken(r.Context(), user.ID.String(), tokenHash, h.Config.JWTTTLRefresh); err != nil {
		slog.Error("failed to store refresh token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	refreshToken := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(h.Config.JWTTTLRefresh),
	}

	if err := h.Repo.CreateRefreshToken(r.Context(), refreshToken); err != nil {
		slog.Error("failed to create refresh token record", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// Refresh handles token refresh
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		http.Error(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	tokenHash := h.Repo.HashRefreshToken(req.RefreshToken)

	userID, err := h.Redis.GetRefreshTokenUserID(r.Context(), tokenHash)
	if err != nil {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		slog.Error("invalid user ID from refresh token", "user_id", userID)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	user, err := h.Repo.GetUserByID(r.Context(), userUUID)
	if err != nil {
		slog.Error("failed to get user", "error", err)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		h.Config.JWTSecret,
		h.Config.JWTIssuer,
		user.ID,
		roleNames,
		h.Config.JWTTTLAccess,
		h.Config.JWTTTLRefresh,
	)
	if err != nil {
		slog.Error("failed to create token pair", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.Redis.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		slog.Error("failed to revoke old refresh token", "error", err)
	}

	newTokenHash := h.Repo.HashRefreshToken(tokens.RefreshToken)
	if err := h.Redis.StoreRefreshToken(r.Context(), user.ID.String(), newTokenHash, h.Config.JWTTTLRefresh); err != nil {
		slog.Error("failed to store new refresh token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	newRefreshToken := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().Add(h.Config.JWTTTLRefresh),
	}

	if err := h.Repo.CreateRefreshToken(r.Context(), newRefreshToken); err != nil {
		slog.Error("failed to create new refresh token record", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// Logout handles user logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken != "" {
		tokenHash := h.Repo.HashRefreshToken(req.RefreshToken)
		if err := h.Redis.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token", "error", err)
		}
		if err := h.Repo.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Error("failed to revoke refresh token in db", "error", err)
		}
	}

	// Blacklist the current access token so it can't be reused after logout
	if claims, ok := auth.FromContext(r.Context()); ok {
		raw := r.Header.Get("Authorization")
		if len(raw) > 7 {
			tokenStr := raw[7:] // strip "Bearer "
			tokenHash := h.Repo.HashRefreshToken(tokenStr)
			ttl := time.Until(claims.ExpiresAt.Time)
			if ttl > 0 {
				if err := h.Redis.StoreBlacklistedToken(r.Context(), tokenHash, ttl); err != nil {
					slog.Error("failed to blacklist access token", "error", err)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "logged out successfully"})
}

