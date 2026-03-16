package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/google/uuid"
)

type ekgAnalyzeRequest struct {
	ImageTempURL string `json:"image_temp_url"`
	Notes        string `json:"notes,omitempty"`
}

// SubmitEKGAnalyze handles EKG image analysis submission
func (h *EKGHandler) SubmitEKGAnalyze(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req ekgAnalyzeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ImageTempURL == "" {
		writeError(w, http.StatusBadRequest, "image_temp_url is required")
		return
	}

	const maxNotesLen = 2000
	if len(req.Notes) > maxNotesLen {
		writeError(w, http.StatusBadRequest, "notes too long")
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Create request record BEFORE enqueueing job so we can return request_id immediately
	requestID := uuid.New()
	request := &models.Request{
		ID:     requestID,
		UserID: userID,
		Status: models.StatusPending,
	}
	if req.Notes != "" {
		request.TextQuery = &req.Notes
	}

	if err := h.Repo.CreateRequest(r.Context(), request); err != nil {
		slog.Error("failed to create request", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	// Create EKG job payload with user ID and request ID
	payload, err := json.Marshal(job.EKGJobPayload{
		ImageTempURL: req.ImageTempURL,
		Notes:        req.Notes,
		UserID:       userID,
		RequestID:    requestID,
	})
	if err != nil {
		slog.Error("failed to marshal EKG payload", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	j := &job.Job{
		Type:    job.TypeEKGAnalyze,
		Payload: payload,
	}
	id, err := h.Queue.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue EKG job", "error", err)
		writeError(w, http.StatusServiceUnavailable, "enqueue failed")
		return
	}

	slog.Info("ekg analysis job enqueued",
		"job_id", id,
		"request_id", requestID,
		"user_id", userID)

	writeJSON(w, http.StatusOK, SubmitEKGResponse{
		JobID:     id,
		RequestID: requestID,
		Status:    string(j.Status),
		Message:   "EKG analysis job submitted successfully",
	})
}
