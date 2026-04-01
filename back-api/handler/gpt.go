package handler

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/fedutinova/smartheart/back-api/service"
	"github.com/fedutinova/smartheart/back-api/validation"
)

// SubmitGPTRequest handles GPT processing request with file uploads.
func (h *GPTHandler) SubmitGPTRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	textQuery := r.FormValue("text_query")
	files := r.MultipartForm.File["files"]

	if validationErrs := validation.ValidateGPTRequest(textQuery, files); len(validationErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, APIError{
			Error:   "validation failed",
			Details: validationErrs,
		})
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	// Convert multipart files to service.UploadedFile
	var uploaded []service.UploadedFile
	var openFiles []multipart.File
	defer func() {
		for _, f := range openFiles {
			_ = f.Close()
		}
	}()
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to open file %s", fh.Filename))
			return
		}
		openFiles = append(openFiles, f)

		uploaded = append(uploaded, service.UploadedFile{
			Reader:      f,
			Filename:    fh.Filename,
			ContentType: fh.Header.Get("Content-Type"),
			Size:        fh.Size,
		})
	}

	result, err := h.Service.SubmitGPT(r.Context(), userID, textQuery, uploaded)
	if err != nil {
		if result != nil && len(result.UploadErrors) > 0 {
			writeJSON(w, http.StatusBadRequest, APIError{
				Error:        "no files successfully processed",
				UploadErrors: result.UploadErrors,
			})
			return
		}
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitGPTResponse{
		RequestID:      result.RequestID,
		JobID:          result.JobID,
		Status:         result.Status,
		FilesProcessed: result.FilesProcessed,
		UploadErrors:   result.UploadErrors,
	})
}
