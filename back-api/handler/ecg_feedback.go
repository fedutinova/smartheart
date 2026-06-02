package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// ECGFeedbackHandler exposes thumbs-style feedback endpoints for the ML
// rhythm conclusion. The handler only persists votes — analytics and
// admin-side aggregation live elsewhere.
type ECGFeedbackHandler struct {
	Repo repository.Store
}

type ecgFeedbackRequest struct {
	Rating  string `json:"rating"            validate:"required,oneof=helpful inaccurate unclear"`
	Comment string `json:"comment,omitempty" validate:"omitempty,max=1000"`
}

type ecgFeedbackResponse struct {
	Rating  string `json:"rating"`
	Comment string `json:"comment,omitempty"`
}

// SubmitFeedback handles POST /v1/ecg/{id}/feedback — upserts a single vote
// per (request, user) pair. Re-submitting overwrites the previous rating.
// Authorisation: the request must belong to the caller. We don't reuse the
// request-service authz path here to avoid pulling another dependency — we
// load the request inline and check user_id.
func (h *ECGFeedbackHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	requestID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	var body ecgFeedbackRequest
	if !decodeAndValidate(w, r, &body) {
		return
	}

	if !h.assertRequestOwner(w, r, requestID, userID) {
		return
	}

	rating := models.ECGFeedbackRating(body.Rating)
	comment := strings.TrimSpace(body.Comment)
	if err := h.Repo.UpsertECGFeedback(r.Context(), &models.ECGFeedback{
		RequestID: requestID,
		UserID:    userID,
		Rating:    rating,
		Comment:   comment,
	}); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ecgFeedbackResponse{Rating: string(rating), Comment: comment})
}

// GetFeedback handles GET /v1/ecg/{id}/feedback — returns 200 with the
// stored rating if the user has voted, 204 otherwise.
func (h *ECGFeedbackHandler) GetFeedback(w http.ResponseWriter, r *http.Request) {
	requestID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	if !h.assertRequestOwner(w, r, requestID, userID) {
		return
	}

	fb, err := h.Repo.GetECGFeedback(r.Context(), requestID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	if fb == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, ecgFeedbackResponse{Rating: string(fb.Rating), Comment: fb.Comment})
}

// assertRequestOwner returns true if requestID exists and belongs to userID.
// Writes the appropriate error response and returns false otherwise.
func (h *ECGFeedbackHandler) assertRequestOwner(w http.ResponseWriter, r *http.Request, requestID, userID uuid.UUID) bool {
	req, err := h.Repo.GetRequestByID(r.Context(), requestID)
	if err != nil {
		handleServiceError(w, err)
		return false
	}
	if req == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return false
	}
	if req.UserID != userID {
		writeError(w, http.StatusForbidden, "request belongs to another user")
		return false
	}
	return true
}
