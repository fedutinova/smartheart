package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/validation"
	"github.com/google/uuid"
)

// SubmitGPTRequest handles GPT processing request with file uploads
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

	request := &models.Request{
		ID:     uuid.New(),
		UserID: userID,
		Status: models.StatusPending,
	}
	if textQuery != "" {
		request.TextQuery = &textQuery
	}

	if err := h.Repo.CreateRequest(r.Context(), request); err != nil {
		slog.Error("failed to create request", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	var fileKeys []string
	var uploadErrors []string
	for _, fileHeader := range files {
		key, err := h.processUploadedFile(r, request.ID, fileHeader)
		if err != nil {
			slog.Error("failed to process file", "filename", fileHeader.Filename, "error", err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("%s: %s", fileHeader.Filename, err.Error()))
			continue
		}
		fileKeys = append(fileKeys, key)
	}

	if len(fileKeys) == 0 {
		// Mark the request as failed so it doesn't stay in "pending" forever.
		if err := h.Repo.UpdateRequestStatus(r.Context(), request.ID, models.StatusFailed); err != nil {
			slog.Error("failed to mark request as failed", "request_id", request.ID, "error", err)
		}
		writeJSON(w, http.StatusBadRequest, APIError{
			Error:        "no files successfully processed",
			UploadErrors: uploadErrors,
		})
		return
	}

	payload := gpt.JobPayload{
		RequestID: request.ID,
		TextQuery: textQuery,
		FileKeys:  fileKeys,
		UserID:    userID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	j := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	jobID, err := h.Queue.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue job", "error", err)
		writeError(w, http.StatusServiceUnavailable, "failed to enqueue job")
		return
	}

	writeJSON(w, http.StatusOK, SubmitGPTResponse{
		RequestID:      request.ID,
		JobID:          jobID,
		Status:         request.Status,
		FilesProcessed: len(fileKeys),
		UploadErrors:   uploadErrors,
	})
}

// processUploadedFile opens, detects content type, uploads and records a single file.
func (h *GPTHandler) processUploadedFile(r *http.Request, requestID uuid.UUID, fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		buf := make([]byte, 512)
		n, readErr := io.ReadFull(file, buf)
		if n == 0 && readErr != nil {
			return "", fmt.Errorf("detect content type: %w", readErr)
		}
		contentType = http.DetectContentType(buf[:n])
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return "", fmt.Errorf("seek: %w", err)
		}
	}

	uploadResult, err := h.Storage.UploadFile(r.Context(), fh.Filename, file, contentType)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	fileModel := &models.File{
		ID:               uuid.New(),
		RequestID:        requestID,
		OriginalFilename: fh.Filename,
		FileType:         contentType,
		FileSize:         fh.Size,
		S3Key:            uploadResult.Key,
		S3URL:            uploadResult.URL,
	}
	if err := h.Repo.CreateFile(r.Context(), fileModel); err != nil {
		return "", fmt.Errorf("create file record: %w", err)
	}

	return uploadResult.Key, nil
}
