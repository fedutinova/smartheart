package handler

import (
	"net/http"
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

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	result, err := h.Service.SubmitEKG(r.Context(), userID, req.ImageTempURL, req.Notes)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitEKGResponse{
		JobID:     result.JobID,
		RequestID: result.RequestID,
		Status:    result.Status,
		Message:   "EKG analysis job submitted successfully",
	})
}
