package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/job"
)

// ECGSyncHandler processes ECG requests synchronously (no queue).
// Activated via ECG_SYNC_MODE=true for H2 baseline comparison.
type ECGSyncHandler struct {
	Worker ECGSyncProcessor
}

// SubmitECGAnalyzeSync handles POST /v1/ecg/analyze in synchronous mode.
func (h *ECGSyncHandler) SubmitECGAnalyzeSync(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer func() { _ = file.Close() }()

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	// RequestID == uuid.Nil signals processEKG to create the request row itself.
	payload := &job.ECGJobPayload{
		ImageFileKey:  "",
		UserID:        userID,
		RequestID:     uuid.Nil,
		PaperSpeedMMS: 25,
		MmPerMvLimb:   10,
		MmPerMvChest:  10,
	}

	if v := r.FormValue("sex"); v != "" {
		payload.Sex = v
	}
	if v := r.FormValue("age"); v != "" {
		if age, err := strconv.Atoi(v); err == nil && age > 0 && age <= 150 {
			payload.Age = &age
		}
	}
	if v := r.FormValue("paper_speed_mms"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 10 && f <= 100 {
			payload.PaperSpeedMMS = f
		}
	}
	if v := r.FormValue("mm_per_mv_limb"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 && f <= 40 {
			payload.MmPerMvLimb = f
		}
	}
	if v := r.FormValue("mm_per_mv_chest"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 && f <= 40 {
			payload.MmPerMvChest = f
		}
	}

	// Upload file to storage first so the worker can read it.
	uploaded, err := h.Worker.UploadForSync(r.Context(), header.Filename, file, header.Header.Get("Content-Type"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to upload image")
		return
	}
	payload.ImageFileKey = uploaded

	// Process synchronously — blocks until GPT completes.
	if err := h.Worker.ProcessECGSync(r.Context(), payload); err != nil {
		writeError(w, http.StatusInternalServerError, "ECG analysis failed")
		return
	}

	// payload.RequestID is populated by ProcessECGSync.
	writeJSON(w, http.StatusOK, SubmitECGResponse{
		RequestID: payload.RequestID,
		Status:    "completed",
		Message:   "ECG analysis completed (sync mode)",
	})
}
