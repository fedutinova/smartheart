package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/ekg"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
)

type EKGHandler struct {
	db           *database.DB
	preprocessor *ekg.EKGPreprocessor
}

// EKGJobPayload represents the payload for EKG analysis jobs
type EKGJobPayload struct {
	ImageTempURL string `json:"image_temp_url"`
	Notes        string `json:"notes,omitempty"`
	UserID       string `json:"user_id,omitempty"`
}

func NewEKGHandler(db *database.DB) *EKGHandler {
	return &EKGHandler{
		db:           db,
		preprocessor: ekg.NewEKGPreprocessor(),
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
	if err := h.saveEKGResults(ctx, j.ID, result, payload); err != nil {
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

func (h *EKGHandler) saveEKGResults(ctx context.Context, jobID uuid.UUID, result *ekg.PreprocessingResult, payload EKGJobPayload) error {
	// Parse user ID
	userUUID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Create a request record for the EKG analysis
	requestID := uuid.New()

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

	// Extract signal features for detailed analysis
	features := h.preprocessor.ExtractSignalFeatures(result.SignalContour)

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

	return nil
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

// mustMarshalJSON marshals data to JSON, panics on error (for mock data only)
func mustMarshalJSON(data interface{}) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "[]"
	}
	return string(bytes)
}
