package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/service"
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

// Login handles user authentication
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, err := h.Service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		handleServiceError(w, err)
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

	tokens, err := h.Service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req tokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Extract access token from Authorization header
	var accessToken string
	if raw := r.Header.Get("Authorization"); strings.HasPrefix(raw, "Bearer ") {
		accessToken = strings.TrimPrefix(raw, "Bearer ")
	}
	claims, _ := auth.FromContext(r.Context())

	_ = h.Service.Logout(r.Context(), req.RefreshToken, accessToken, claims)

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// handleServiceError maps service-layer errors to HTTP responses.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTooManyAttempts):
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
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
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
