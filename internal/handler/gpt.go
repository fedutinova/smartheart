package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/validation"
	"github.com/google/uuid"
)

// SubmitGPTRequest handles GPT processing request with file uploads
func (h *Handlers) SubmitGPTRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	textQuery := r.FormValue("text_query")
	files := r.MultipartForm.File["files"]

	if validationErrs := validation.ValidateGPTRequest(textQuery, files); len(validationErrs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "validation failed",
			"details": validationErrs,
		})
		return
	}

	var userID uuid.UUID
	if claims, ok := auth.FromContext(r.Context()); ok {
		var err error
		userID, err = uuid.Parse(claims.UserID)
		if err != nil {
			http.Error(w, "invalid user ID", http.StatusBadRequest)
			return
		}
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
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	var fileKeys []string
	for _, fileHeader := range files {
		key, err := h.processUploadedFile(r, request.ID, fileHeader)
		if err != nil {
			slog.Error("failed to process file", "filename", fileHeader.Filename, "error", err)
			continue
		}
		fileKeys = append(fileKeys, key)
	}

	if len(fileKeys) == 0 {
		http.Error(w, "no files successfully processed", http.StatusBadRequest)
		return
	}

	payload := gpt.GPTJobPayload{
		RequestID: request.ID,
		TextQuery: textQuery,
		FileKeys:  fileKeys,
		UserID:    userID.String(),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	j := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	jobID, err := h.Q.Enqueue(r.Context(), j)
	if err != nil {
		slog.Error("failed to enqueue job", "error", err)
		http.Error(w, "failed to enqueue job", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"request_id":      request.ID,
		"job_id":          jobID,
		"status":          request.Status,
		"files_processed": len(fileKeys),
	})
}

// processUploadedFile opens, detects content type, uploads and records a single file.
// Extracted from the loop to ensure file handles are closed promptly via defer.
func (h *Handlers) processUploadedFile(r *http.Request, requestID uuid.UUID, fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		// Read first 512 bytes for detection, then reset reader.
		buf := make([]byte, 512)
		n, _ := io.ReadFull(file, buf)
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

