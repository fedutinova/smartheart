package handler

import (
	"net/http"

	"github.com/fedutinova/smartheart/back-api/repository"
)

// ProfileHandler handles user profile endpoints.
type ProfileHandler struct {
	Repo repository.Store
}

// GetMe returns the current user's profile.
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.Repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	roles := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roles[i] = r.Name
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"roles":      roles,
		"created_at": user.CreatedAt,
	})
}
