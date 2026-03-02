package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/google/uuid"
)

type EKGHandler struct {
	db      *database.DB
	queue   memq.JobQueue
	storage storage.Storage
	repo    *repository.Repository
}

func NewEKGHandler(db *database.DB, queue memq.JobQueue, storageService storage.Storage, repo *repository.Repository) *EKGHandler {
	return &EKGHandler{
		db:      db,
		queue:   queue,
		storage: storageService,
		repo:    repo,
	}
}

func (h *EKGHandler) HandleEKGJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeEKGAnalyze {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload job.EKGJobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal EKG job payload: %w", err)
	}

	slog.Info("Starting EKG analysis",
		"job_id", j.ID,
		"image_url", payload.ImageTempURL,
		"user_id", payload.UserID)

	// Download image from temp URL
	imageData, err := h.downloadImage(ctx, payload.ImageTempURL)
	if err != nil {
		slog.Error("Failed to download EKG image", "job_id", j.ID, "error", err)
		return fmt.Errorf("failed to download image: %w", err)
	}

	// Save results and trigger GPT analysis directly — no OpenCV preprocessing
	if err := h.saveEKGResults(ctx, j.ID, payload, imageData); err != nil {
		return fmt.Errorf("failed to save EKG results: %w", err)
	}

	slog.Info("EKG analysis job completed, GPT analysis triggered",
		"job_id", j.ID)

	return nil
}

// validateImageURL checks that the URL is safe to fetch (prevents SSRF)
func validateImageURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty image URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow http and https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http/https allowed)", parsed.Scheme)
	}

	hostname := parsed.Hostname()

	// Block localhost and loopback
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "0.0.0.0" {
		return fmt.Errorf("requests to localhost are not allowed")
	}

	// Resolve hostname and check for private/reserved IP ranges
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("requests to private/reserved IP addresses are not allowed")
		}
	}

	return nil
}

// downloadImage downloads image from URL with timeout and size limits
func (h *EKGHandler) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	if err := validateImageURL(imageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	const maxImageSize = 10 * 1024 * 1024 // 10MB

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "SmartHeart-EKG-Processor/1.0")
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isValidImageContentType(contentType) {
		return nil, fmt.Errorf("invalid content type: %s", contentType)
	}

	if resp.ContentLength > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes (max %d)", resp.ContentLength, maxImageSize)
	}

	// Use LimitReader to prevent unbounded memory consumption
	imageData, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("image too large after download: %d bytes (max %d)", len(imageData), maxImageSize)
	}

	slog.Debug("Downloaded EKG image",
		"url", imageURL,
		"content_type", contentType,
		"size_bytes", len(imageData))

	return imageData, nil
}

// isValidImageContentType checks if the content type is a valid image format
func isValidImageContentType(contentType string) bool {
	validTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"image/bmp":       true,
		"image/tiff":      true,
		"application/pdf": true, // PDFs can contain images
	}
	return validTypes[contentType]
}

func (h *EKGHandler) saveEKGResults(ctx context.Context, jobID uuid.UUID, payload job.EKGJobPayload, imageData []byte) error {
	// Parse user ID
	userUUID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Get or create request ID
	var requestID uuid.UUID
	if payload.RequestID != "" {
		requestID, err = uuid.Parse(payload.RequestID)
		if err != nil {
			return fmt.Errorf("invalid request ID: %w", err)
		}
	} else {
		requestID = uuid.New()
	}

	// Trigger GPT analysis: upload image and create GPT job
	gptRequestID, err := h.triggerGPTAnalysis(ctx, jobID, requestID, payload, userUUID, imageData)
	if err != nil {
		slog.Warn("Failed to trigger GPT analysis", "error", err, "job_id", jobID)
		gptRequestID = uuid.Nil
	}

	// Create response content
	ekgContent := &models.EKGResponseContent{
		AnalysisType: models.EKGModelDirect,
		Notes:        payload.Notes,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		JobID:        jobID.String(),
	}
	if gptRequestID != uuid.Nil {
		ekgContent.GPTRequestID = gptRequestID.String()
		ekgContent.GPTInterpretationStatus = "pending"
	}

	responseJSON, err := ekgContent.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal response content: %w", err)
	}

	// Use transaction to ensure atomicity
	err = h.db.WithTx(ctx, func(tx database.Tx) error {
		txRepo := h.repo.WithTx(tx)

		if payload.RequestID != "" {
			if err := txRepo.UpdateRequestStatus(ctx, requestID, models.StatusProcessing); err != nil {
				return fmt.Errorf("failed to update request status to processing: %w", err)
			}
		} else {
			request := &models.Request{
				ID:     requestID,
				UserID: userUUID,
				Status: models.StatusCompleted,
			}
			if payload.Notes != "" {
				request.TextQuery = &payload.Notes
			}
			if err := txRepo.CreateRequest(ctx, request); err != nil {
				return fmt.Errorf("failed to create request record: %w", err)
			}
		}

		response := &models.Response{
			ID:               uuid.New(),
			RequestID:        requestID,
			Content:          responseJSON,
			Model:            models.EKGModelDirect,
			TokensUsed:       0,
			ProcessingTimeMs: 0,
		}
		if err := txRepo.CreateResponse(ctx, response); err != nil {
			return fmt.Errorf("failed to save EKG response: %w", err)
		}

		if err := txRepo.UpdateRequestStatus(ctx, requestID, models.StatusCompleted); err != nil {
			return fmt.Errorf("failed to update request status to completed: %w", err)
		}

		slog.Info("Saved EKG analysis results",
			"job_id", jobID,
			"request_id", requestID,
			"response_id", response.ID)

		return nil
	})

	return err
}

// triggerGPTAnalysis uploads the image and creates a GPT job for EKG interpretation
func (h *EKGHandler) triggerGPTAnalysis(ctx context.Context, ekgJobID uuid.UUID, ekgRequestID uuid.UUID, payload job.EKGJobPayload, userUUID uuid.UUID, imageData []byte) (uuid.UUID, error) {
	filename := fmt.Sprintf("ekg_%s.jpg", ekgJobID.String()[:8])
	uploadResult, err := h.storage.UploadFile(ctx, filename, bytes.NewReader(imageData), "image/jpeg")
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upload image: %w", err)
	}

	gptRequestID := uuid.New()

	// Build EKG-specific prompt — GPT-4o analyzes the image directly
	var textQuery string
	if payload.Notes != "" {
		textQuery = fmt.Sprintf(`Analyze this ECG/EKG image. Describe what you observe in Russian language.

Structure your analysis:
1. Качество изображения: четкость, наличие артефактов, видимость отведений
2. Ритм: регулярный/нерегулярный, приблизительная ЧСС если видна разметка
3. Зубцы и интервалы: P, QRS, T — форма, амплитуда, длительность (если видна калибровка)
4. Сегменты: ST-сегмент (элевация/депрессия), PR-интервал, QT-интервал
5. Особенности: любые отклонения от нормального синусового ритма

Additional context from user:
%s

This is for educational and technical analysis. Describe observations without making diagnostic conclusions.`, payload.Notes)
	} else {
		textQuery = `Analyze this ECG/EKG image. Describe what you observe in Russian language.

Structure your analysis:
1. Качество изображения: четкость, наличие артефактов, видимость отведений
2. Ритм: регулярный/нерегулярный, приблизительная ЧСС если видна разметка
3. Зубцы и интервалы: P, QRS, T — форма, амплитуда, длительность (если видна калибровка)
4. Сегменты: ST-сегмент (элевация/депрессия), PR-интервал, QT-интервал
5. Особенности: любые отклонения от нормального синусового ритма

This is for educational and technical analysis. Describe observations without making diagnostic conclusions.`
	}

	gptRequest := &models.Request{
		ID:        gptRequestID,
		UserID:    userUUID,
		TextQuery: &textQuery,
		Status:    models.StatusPending,
	}

	if err := h.repo.CreateRequest(ctx, gptRequest); err != nil {
		return uuid.Nil, fmt.Errorf("failed to create GPT request: %w", err)
	}

	fileModel := &models.File{
		ID:               uuid.New(),
		RequestID:        gptRequestID,
		OriginalFilename: filename,
		FileType:         "image/jpeg",
		FileSize:         int64(len(imageData)),
		S3Key:            uploadResult.Key,
		S3URL:            uploadResult.URL,
	}

	if err := h.repo.CreateFile(ctx, fileModel); err != nil {
		return uuid.Nil, fmt.Errorf("failed to create file record: %w", err)
	}

	gptPayload := gpt.GPTJobPayload{
		RequestID: gptRequestID,
		TextQuery: textQuery,
		FileKeys:  []string{uploadResult.Key},
		UserID:    payload.UserID,
	}

	payloadBytes, err := json.Marshal(gptPayload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal GPT payload: %w", err)
	}

	gptJob := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	gptJobID, err := h.queue.Enqueue(ctx, gptJob)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to enqueue GPT job: %w", err)
	}

	slog.Info("GPT analysis job triggered for EKG image",
		"ekg_job_id", ekgJobID,
		"ekg_request_id", ekgRequestID,
		"gpt_job_id", gptJobID,
		"gpt_request_id", gptRequestID,
		"image_key", uploadResult.Key)

	return gptRequestID, nil
}

// Close cleans up resources used by the EKG handler
func (h *EKGHandler) Close() {
	slog.Debug("EKG handler closed")
}
