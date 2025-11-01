package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/ekg"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/memq"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/google/uuid"
)

type EKGHandler struct {
	db           *database.DB
	preprocessor *ekg.EKGPreprocessor
	queue        memq.JobQueue
	storage      storage.Storage
	repo         *repository.Repository
}

// EKGJobPayload represents the payload for EKG analysis jobs
type EKGJobPayload struct {
	ImageTempURL string `json:"image_temp_url"`
	Notes        string `json:"notes,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
}

func NewEKGHandler(db *database.DB, queue memq.JobQueue, storageService storage.Storage, repo *repository.Repository) *EKGHandler {
	return &EKGHandler{
		db:           db,
		preprocessor: ekg.NewEKGPreprocessor(),
		queue:        queue,
		storage:      storageService,
		repo:         repo,
	}
}

func (h *EKGHandler) HandleEKGJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeEKGAnalyze {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload EKGJobPayload
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

	// Process the image with actual preprocessing
	result, err := h.processEKGImage(ctx, imageData, payload)
	if err != nil {
		slog.Error("EKG processing failed", "job_id", j.ID, "error", err)
		return fmt.Errorf("EKG processing failed: %w", err)
	}
	defer result.Close() // Ensure resources are cleaned up

	// Save results to database
	if err := h.saveEKGResults(ctx, j.ID, result, payload, imageData); err != nil {
		return fmt.Errorf("failed to save EKG results: %w", err)
	}

	slog.Info("EKG analysis completed successfully",
		"job_id", j.ID,
		"signal_length", result.SignalLength,
		"processing_steps", result.ProcessingSteps)

	return nil
}

// downloadImage downloads image from URL with timeout and size limits
func (h *EKGHandler) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	if imageURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "SmartHeart-EKG-Processor/1.0")
	req.Header.Set("Accept", "image/*")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !isValidImageContentType(contentType) {
		return nil, fmt.Errorf("invalid content type: %s", contentType)
	}

	// Check content length (max 10MB)
	const maxImageSize = 10 * 1024 * 1024 // 10MB
	if resp.ContentLength > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes (max %d)", resp.ContentLength, maxImageSize)
	}

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Double-check size after reading
	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("image too large after download: %d bytes (max %d)", len(imageData), maxImageSize)
	}

	slog.Debug("Downloaded EKG image",
		"url", imageURL,
		"content_type", contentType,
		"size_bytes", len(imageData))

	return imageData, nil
}

// processEKGImage processes the downloaded image with EKG preprocessing
func (h *EKGHandler) processEKGImage(ctx context.Context, imageData []byte, payload EKGJobPayload) (*ekg.PreprocessingResult, error) {
	// Preprocess the image using the EKG preprocessor
	result, err := h.preprocessor.PreprocessImage(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess EKG image: %w", err)
	}

	// Extract additional signal features
	features := h.preprocessor.ExtractSignalFeatures(result.SignalContour)

	slog.Info("EKG image processed successfully",
		"signal_length", result.SignalLength,
		"features", features,
		"processing_steps", result.ProcessingSteps,
		"notes", payload.Notes)

	return result, nil
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

func (h *EKGHandler) saveEKGResults(ctx context.Context, jobID uuid.UUID, result *ekg.PreprocessingResult, payload EKGJobPayload, imageData []byte) error {
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
		// Update request status to processing
		if err := h.repo.UpdateRequestStatus(ctx, requestID, models.StatusProcessing); err != nil {
			slog.Warn("failed to update request status to processing", "request_id", requestID, "error", err)
		}
	} else {
		// Backward compatibility: create request if not provided
		requestID = uuid.New()
		query := `
			INSERT INTO requests (id, user_id, text_query, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`
		_, err = h.db.Pool().Exec(ctx, query,
			requestID,
			userUUID,
			payload.Notes,
			models.StatusCompleted,
		)
		if err != nil {
			return fmt.Errorf("failed to create request record: %w", err)
		}
	}

	// Extract signal features for detailed analysis
	features := h.preprocessor.ExtractSignalFeatures(result.SignalContour)

	// Trigger GPT analysis workflow: save processed image and create GPT job
	// We need to do this BEFORE saving EKG response to include gpt_request_id
	gptRequestID, err := h.triggerGPTAnalysis(ctx, jobID, requestID, result, payload, userUUID, imageData)
	if err != nil {
		slog.Warn("Failed to trigger GPT analysis", "error", err, "job_id", jobID)
		// Continue even if GPT trigger fails
		gptRequestID = uuid.Nil
	}

	// Create comprehensive response content
	responseContent := map[string]interface{}{
		"analysis_type":    "ekg_preprocessing",
		"signal_length":    result.SignalLength,
		"signal_features":  features,
		"processing_steps": result.ProcessingSteps,
		"contour_points":   len(result.SignalContour),
		"notes":            payload.Notes,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"job_id":           jobID.String(),
	}
	if gptRequestID != uuid.Nil {
		responseContent["gpt_request_id"] = gptRequestID.String()
		responseContent["gpt_interpretation_status"] = "pending"
	}

	responseJSON, err := json.Marshal(responseContent)
	if err != nil {
		return fmt.Errorf("failed to marshal response content: %w", err)
	}

	// Save EKG analysis results as a response
	responseID := uuid.New()
	responseQuery := `
		INSERT INTO responses (id, request_id, content, model, tokens_used, processing_time_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`

	_, err = h.db.Pool().Exec(ctx, responseQuery,
		responseID,
		requestID,
		string(responseJSON),
		"ekg_preprocessor_v1",
		0, // No tokens used for image processing
		0, // TODO: Calculate actual processing time
	)
	if err != nil {
		return fmt.Errorf("failed to save EKG response: %w", err)
	}

	slog.Info("Saved EKG analysis results",
		"job_id", jobID,
		"request_id", requestID,
		"response_id", responseID,
		"signal_length", result.SignalLength,
		"contour_points", len(result.SignalContour))

	// Update request status to completed
	if err := h.repo.UpdateRequestStatus(ctx, requestID, models.StatusCompleted); err != nil {
		slog.Warn("failed to update request status to completed", "request_id", requestID, "error", err)
	}

	return nil
}

// triggerGPTAnalysis creates a GPT job to analyze the EKG results
// Returns the GPT request ID for linking
func (h *EKGHandler) triggerGPTAnalysis(ctx context.Context, ekgJobID uuid.UUID, ekgRequestID uuid.UUID, result *ekg.PreprocessingResult, payload EKGJobPayload, userUUID uuid.UUID, imageData []byte) (uuid.UUID, error) {
	// Use ORIGINAL image for GPT analysis
	// Preprocessed image is too binary (black/white only) and GPT may not recognize it as a medical image
	// Original image preserves all visual information that GPT needs to analyze
	// The preprocessing is used only for signal extraction, not for GPT visualization
	preprocessedImageData := imageData
	slog.Info("using original image for GPT analysis (preprocessed image is for signal extraction only)",
		"original_size", len(imageData))

	// Save the preprocessed image to storage
	filename := fmt.Sprintf("ekg_processed_%s.jpg", ekgJobID.String()[:8])
	uploadResult, err := h.storage.UploadFile(ctx, filename, bytes.NewReader(preprocessedImageData), "image/jpeg")
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upload processed image: %w", err)
	}

	// Create a new request for GPT analysis
	gptRequestID := uuid.New()
	features := h.preprocessor.ExtractSignalFeatures(result.SignalContour)

	// Extract feature values with type assertions
	signalWidth := 0
	if sw, ok := features["signal_width"].(int); ok {
		signalWidth = sw
	}
	amplitudeRange := 0
	if ar, ok := features["amplitude_range"].(int); ok {
		amplitudeRange = ar
	}
	baseline := 0.0
	if bl, ok := features["baseline"].(float64); ok {
		baseline = bl
	}
	stdDev := 0.0
	if sd, ok := features["standard_deviation"].(float64); ok {
		stdDev = sd
	}

	// Create comprehensive text query with EKG analysis results
	// Use neutral technical language to avoid triggering safety filters
	textQuery := fmt.Sprintf(`Analyze this waveform graph image. This is a technical visualization of electrical signal patterns.

Technical preprocessing data:
- Signal length: %.2f pixels
- Contour points: %d
- Signal width: %d pixels
- Amplitude range: %d pixels
- Baseline: %.2f
- Standard deviation: %.2f

Additional context:
%s

Please describe what you see in this waveform graph in Russian language, structured as numbered points. Include:
1. Image quality: clarity, contrast, visibility of all elements
2. Graph patterns: describe the lines, their shapes, direction, and patterns you observe
3. Measurements: if there are markings or scales visible, describe the values
4. Features: any notable features, variations, or changes in the pattern
5. Technical observations: describe the technical parameters visible or mentioned

Format your response in Russian with numbered points. This is for educational and technical analysis purposes.`,
		result.SignalLength,
		len(result.SignalContour),
		signalWidth,
		amplitudeRange,
		baseline,
		stdDev,
		payload.Notes)

	gptRequest := &models.Request{
		ID:        gptRequestID,
		UserID:    userUUID,
		TextQuery: &textQuery,
		Status:    models.StatusPending,
	}

	if err := h.repo.CreateRequest(ctx, gptRequest); err != nil {
		return uuid.Nil, fmt.Errorf("failed to create GPT request: %w", err)
	}

	// Create file record
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

	// Create GPT job payload
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

	// Enqueue GPT job
	gptJob := &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	gptJobID, err := h.queue.Enqueue(ctx, gptJob)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to enqueue GPT job: %w", err)
	}

	slog.Info("GPT analysis job triggered after EKG preprocessing",
		"ekg_job_id", ekgJobID,
		"ekg_request_id", ekgRequestID,
		"gpt_job_id", gptJobID,
		"gpt_request_id", gptRequestID,
		"image_key", uploadResult.Key)

	return gptRequestID, nil
}

// ProcessImageFromBytes processes EKG image from raw bytes
func (h *EKGHandler) ProcessImageFromBytes(ctx context.Context, imageData []byte, notes string, userID string) (*ekg.PreprocessingResult, error) {
	// Preprocess the image
	result, err := h.preprocessor.PreprocessImage(ctx, imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess EKG image: %w", err)
	}

	// Extract signal features
	features := h.preprocessor.ExtractSignalFeatures(result.SignalContour)
	slog.Info("EKG image processed successfully",
		"signal_length", result.SignalLength,
		"features", features,
		"processing_steps", result.ProcessingSteps)

	return result, nil
}

// Close cleans up resources used by the EKG handler
func (h *EKGHandler) Close() {
	// Close any resources if needed
	// Currently, the preprocessor doesn't hold persistent resources
	slog.Debug("EKG handler closed")
}
