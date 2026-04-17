package handler

import (
	"net/http"

	"github.com/fedutinova/smartheart/back-api/service"
)

type PasswordHandler struct {
	Service service.PasswordService
}

type requestResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type confirmResetRequest struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=10,max=72"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=10,max=72"`
}

func (h *PasswordHandler) RequestReset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req requestResetRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	if err := h.Service.RequestReset(r.Context(), req.Email); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if the email exists, a reset link has been sent",
	})
}

func (h *PasswordHandler) ConfirmReset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req confirmResetRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	if err := h.Service.ConfirmReset(r.Context(), req.Token, req.NewPassword); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password has been reset successfully",
	})
}

func (h *PasswordHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req changePasswordRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	if err := h.Service.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password changed successfully",
	})
}
