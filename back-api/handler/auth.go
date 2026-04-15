package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/service"
)

const maxBodySize = 1 << 20 // 1 MB

type registerRequest struct {
	Username string `json:"username" validate:"required,max=100"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=10,max=72"`
}

type loginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// accessTokenResponse is the JSON body returned by login/refresh.
// The refresh token is no longer included — it travels as an httpOnly cookie.
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Register handles user registration.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req registerRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	userID, err := h.Service.Register(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		Message: "user registered successfully",
		UserID:  userID,
	})
}

// Login handles user authentication.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req loginRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	tokens, err := h.Service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	auth.SetRefreshTokenCookie(w, tokens.RefreshToken, h.Config.JWT.TTLRefresh, h.Config.Cookie)
	writeJSON(w, http.StatusOK, accessTokenResponse{AccessToken: tokens.AccessToken})
}

// Refresh handles token refresh.
// The refresh token is read from the httpOnly cookie.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := auth.RefreshTokenFromCookie(r)
	if refreshToken == "" {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	tokens, err := h.Service.Refresh(r.Context(), refreshToken)
	if err != nil {
		auth.ClearRefreshTokenCookie(w, h.Config.Cookie)
		handleServiceError(w, err)
		return
	}

	auth.SetRefreshTokenCookie(w, tokens.RefreshToken, h.Config.JWT.TTLRefresh, h.Config.Cookie)
	writeJSON(w, http.StatusOK, accessTokenResponse{AccessToken: tokens.AccessToken})
}

// Logout handles user logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := auth.RefreshTokenFromCookie(r)

	// Extract access token from Authorization header
	var accessToken string
	if raw := r.Header.Get("Authorization"); strings.HasPrefix(raw, "Bearer ") {
		accessToken = strings.TrimPrefix(raw, "Bearer ")
	}
	claims, _ := auth.FromContext(r.Context())

	_ = h.Service.Logout(r.Context(), refreshToken, accessToken, claims)

	auth.ClearRefreshTokenCookie(w, h.Config.Cookie)
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// handleServiceError maps service-layer errors to HTTP responses.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTooManyAttempts):
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
	case errors.Is(err, apperr.ErrPaymentRequired):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case apperr.IsValidation(err):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, apperr.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, apperr.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid token")
	case apperr.IsConflict(err):
		writeError(w, http.StatusConflict, "already exists")
	case apperr.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not found")
	case apperr.IsForbidden(err):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		slog.Error("Unhandled service error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
