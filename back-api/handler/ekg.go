package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/fedutinova/smartheart/back-api/service"
)

type ekgAnalyzeRequest struct {
	ImageTempURL string `json:"image_temp_url" validate:"required,url"`
	Notes        string `json:"notes,omitempty" validate:"max=2000"`
}

// SubmitEKGAnalyze handles EKG image analysis submission.
// Accepts either JSON (URL mode) or multipart/form-data (file upload mode).
func (h *EKGHandler) SubmitEKGAnalyze(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.submitEKGFile(w, r)
		return
	}

	h.submitEKGURL(w, r)
}

// submitEKGURL handles URL-based EKG submission (existing behavior).
func (h *EKGHandler) submitEKGURL(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req ekgAnalyzeRequest
	if !decodeAndValidate(w, r, &req) {
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

// submitEKGFile handles file-based EKG submission (multipart upload).
func (h *EKGHandler) submitEKGFile(w http.ResponseWriter, r *http.Request) {
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
	defer file.Close()

	notes := r.FormValue("notes")

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	uploaded := service.UploadedFile{
		Reader:      file,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
	}

	result, err := h.Service.SubmitEKGFile(r.Context(), userID, uploaded, notes)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitEKGResponse{
		JobID:     result.JobID,
		RequestID: result.RequestID,
		Status:    result.Status,
		Message:   fmt.Sprintf("EKG analysis job submitted successfully (file: %s)", header.Filename),
	})
}
