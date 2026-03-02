package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
)

// SubmitEKGAnalyze handles EKG image analysis submission
func (h *Handlers) SubmitEKGAnalyze(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req struct {
		ImageTempURL string `json:"image_temp_url"`
		Notes        string `json:"notes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ImageTempURL == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	var userID string
	if claims, ok := auth.FromContext(r.Context()); ok {
		userID = claims.UserID
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}

	// Create request record BEFORE enqueueing job so we can return request_id immediately
	requestID := uuid.New()
	request := &models.Request{
		ID:     requestID,
		UserID: userUUID,
		Status: models.StatusPending,
	}
	if req.Notes != "" {
		request.TextQuery = &req.Notes
	}

	if err := h.Repo.CreateRequest(r.Context(), request); err != nil {
		slog.Error("failed to create request", "error", err)
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	// Create EKG job payload with user ID and request ID
	payload, err := json.Marshal(job.EKGJobPayload{
		ImageTempURL: req.ImageTempURL,
		Notes:        req.Notes,
		UserID:       userID,
		RequestID:    requestID.String(),
	})
	if err != nil {
		slog.Error("failed to marshal EKG payload", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	j := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: payload,
	}
	id, err := h.Q.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue EKG job", "error", err)
		http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
		return
	}

	slog.Info("EKG analysis job enqueued",
		"job_id", id,
		"request_id", requestID,
		"user_id", userID,
		"image_url", req.ImageTempURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SubmitEKGResponse{
		JobID:     id.String(),
		RequestID: requestID.String(),
		Status:    string(j.Status),
		Message:   "EKG analysis job submitted successfully",
	})
}

