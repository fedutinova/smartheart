package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/fedutinova/smartheart/back-api/validation"
)

// EKGWorker processes EKG analysis jobs.
type EKGWorker struct {
	txb       database.TxBeginner
	queue     job.Queue
	storage   storage.Storage
	repo      repository.RequestRepo
	quotaRepo repository.QuotaRepo
	gptClient gpt.Processor
	hub       *notify.Hub
}

func NewEKGWorker(
	txb database.TxBeginner,
	queue job.Queue,
	storageService storage.Storage,
	repo repository.Store,
	gptClient gpt.Processor,
	hub *notify.Hub,
) *EKGWorker {
	return &EKGWorker{
		txb:       txb,
		queue:     queue,
		storage:   storageService,
		repo:      repo,
		quotaRepo: repo,
		gptClient: gptClient,
		hub:       hub,
	}
}

//nolint:gocognit // orchestration function with inherent branching
func (h *EKGWorker) HandleEKGJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeEKGAnalyze {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload job.EKGJobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal EKG job payload: %w", err)
	}

	err := h.processEKG(ctx, j, &payload)
	if err != nil {
		// Refund the daily usage counter so failed analyses don't count
		if decErr := h.quotaRepo.DecrementDailyUsage(ctx, payload.UserID); decErr != nil {
			slog.WarnContext(ctx, "Failed to decrement daily usage after EKG failure", "user_id", payload.UserID, "error", decErr)
		}
		// Mark request as failed and notify user
		if payload.RequestID != uuid.Nil {
			if updErr := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updErr != nil {
				slog.ErrorContext(ctx, "Failed to update request status to failed", "request_id", payload.RequestID, "error", updErr)
			}
			h.hub.Notify(payload.UserID, notify.Event{
				Type:      "request_completed",
				RequestID: payload.RequestID,
				Status:    models.StatusFailed,
			})
		}
	}
	return err
}

func (h *EKGWorker) processEKG(ctx context.Context, j *job.Job, payload *job.EKGJobPayload) error {

	slog.InfoContext(ctx, "Starting EKG analysis",
		"job_id", j.ID,
		"user_id", payload.UserID,
		"mode", ekgJobMode(payload))

	// Apply defaults
	if payload.PaperSpeedMMS <= 0 {
		payload.PaperSpeedMMS = 25
	}
	if payload.MmPerMvLimb <= 0 {
		payload.MmPerMvLimb = 10
	}
	if payload.MmPerMvChest <= 0 {
		payload.MmPerMvChest = 10
	}

	// Get image
	var imageData []byte
	var err error
	if payload.ImageFileKey != "" {
		imageData, err = h.readFromStorage(ctx, payload.ImageFileKey)
	} else {
		imageData, err = h.downloadImage(ctx, payload.ImageTempURL)
	}
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get EKG image", "job_id", j.ID, "error", err)
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Ensure image is in storage (for file record and GPT access)
	imageKey := payload.ImageFileKey
	imageURL := ""
	if imageKey == "" {
		filename := fmt.Sprintf("ekg_%s.jpg", j.ID.String()[:8])
		uploadResult, uploadErr := h.storage.UploadFile(ctx, filename, bytes.NewReader(imageData), "image/jpeg")
		if uploadErr != nil {
			return fmt.Errorf("failed to upload image: %w", uploadErr)
		}
		imageKey = uploadResult.Key
		imageURL = uploadResult.URL
	}

	// Build prompt and call GPT
	systemPrompt, userPrompt := gpt.BuildECGMeasurementPrompt(payload.PaperSpeedMMS)
	gptResult, err := h.gptClient.ProcessStructuredECG(ctx, []string{imageKey}, systemPrompt, userPrompt)
	if err != nil {
		slog.ErrorContext(ctx, "GPT structured ECG call failed", "job_id", j.ID, "error", err)
		return fmt.Errorf("gpt analysis failed: %w", err)
	}

	// Parse GPT JSON response
	rawMeasurements, err := gpt.ParseECGMeasurementJSON(gptResult.Content)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse GPT ECG JSON", "job_id", j.ID, "error", err)
		return fmt.Errorf("parse GPT response: %w", err)
	}

	// Post-process: convert small squares to mm/ms
	msPerSq := 1000.0 / payload.PaperSpeedMMS
	// Adjust if GPT detected different calibration
	if rawMeasurements.Calibration.PaperSpeed != nil && *rawMeasurements.Calibration.PaperSpeed > 0 {
		detectedMMS := *rawMeasurements.Calibration.PaperSpeed
		if detectedMMS > 10 && detectedMMS < 100 {
			msPerSq = 1000.0 / detectedMMS
		}
	}

	measMM := finalizeFromCounts(rawMeasurements, msPerSq)
	clampMeasurements(measMM)

	timestamp := time.Now().UTC().Format(time.RFC3339)
	structured := computeStructuredResult(
		measMM, payload.Sex, payload.Age,
		payload.MmPerMvLimb, payload.MmPerMvChest,
		timestamp, j.ID.String(),
	)

	// Build response content
	ekgContent := &models.EKGResponseContent{
		AnalysisType:     models.EKGModelStructured,
		Notes:            payload.Notes,
		Timestamp:        timestamp,
		JobID:            j.ID.String(),
		StructuredResult: structured,
	}
	responseJSON, err := ekgContent.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Persist in transaction
	requestID := payload.RequestID
	if requestID == uuid.Nil {
		requestID = uuid.New()
	}

	if err := h.txb.WithTx(ctx, func(tx database.Tx) error {
		txRepo := repository.NewTxScoped(tx)

		if payload.RequestID == uuid.Nil {
			request := &models.Request{
				ID:     requestID,
				UserID: payload.UserID,
				Status: models.StatusCompleted,
			}
			if payload.Notes != "" {
				request.TextQuery = &payload.Notes
			}
			if err := txRepo.CreateRequest(ctx, request); err != nil {
				return fmt.Errorf("create request: %w", err)
			}
		}

		response := &models.Response{
			ID:               uuid.New(),
			RequestID:        requestID,
			Content:          responseJSON,
			Model:            models.EKGModelStructured,
			TokensUsed:       gptResult.TokensUsed,
			ProcessingTimeMs: gptResult.ProcessingTimeMs,
		}
		if err := txRepo.CreateResponse(ctx, response); err != nil {
			return fmt.Errorf("save response: %w", err)
		}

		// Create file record
		fileModel := &models.File{
			ID:               uuid.New(),
			RequestID:        requestID,
			OriginalFilename: fmt.Sprintf("ekg_%s.jpg", j.ID.String()[:8]),
			FileType:         "image/jpeg",
			FileSize:         int64(len(imageData)),
			S3Key:            imageKey,
			S3URL:            imageURL,
		}
		if err := txRepo.CreateFile(ctx, fileModel); err != nil {
			return fmt.Errorf("create file record: %w", err)
		}

		if err := txRepo.UpdateRequestStatus(ctx, requestID, models.StatusCompleted); err != nil {
			return fmt.Errorf("update request status: %w", err)
		}

		slog.InfoContext(ctx, "Saved structured EKG results",
			"job_id", j.ID, "request_id", requestID)
		return nil
	}); err != nil {
		return err
	}

	// Notify frontend
	if h.hub != nil {
		h.hub.Notify(payload.UserID, notify.Event{
			Type:      "request_completed",
			RequestID: requestID,
			Status:    models.StatusCompleted,
		})
	}

	slog.InfoContext(ctx, "EKG structured analysis completed", "job_id", j.ID)
	return nil
}

// Close cleans up resources used by the EKG worker.
func (*EKGWorker) Close() {
	slog.Debug("EKG worker closed")
}

// --- Image download/read helpers (unchanged) ---

func validateImageURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("empty image URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	hostname := parsed.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "0.0.0.0" {
		return errors.New("requests to localhost are not allowed")
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

var sharedSSRFTransport = newSSRFSafeTransport()

func newSSRFSafeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %w", err)
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
			}
			for _, ipAddr := range ips {
				if isPrivateIP(ipAddr.IP) {
					return nil, fmt.Errorf("connections to private/reserved IP %s are not allowed", ipAddr.IP)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		DisableKeepAlives: true,
	}
}

func (*EKGWorker) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	if err := validateImageURL(imageURL); err != nil {
		return nil, fmt.Errorf("url validation failed: %w", err)
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: sharedSSRFTransport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "SmartHeart-EKG-Processor/1.0")
	req.Header.Set("Accept", "image/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isValidImageContentType(contentType) {
		return nil, fmt.Errorf("invalid content type: %s", contentType)
	}

	if resp.ContentLength > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes", resp.ContentLength)
	}

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes", len(imageData))
	}

	return imageData, nil
}

func isValidImageContentType(contentType string) bool {
	return validation.IsImageType(contentType) || contentType == "application/pdf"
}

func ekgJobMode(p *job.EKGJobPayload) string {
	if p.ImageFileKey != "" {
		return "file"
	}
	return "url"
}

const maxImageSize = 10 * 1024 * 1024

func (h *EKGWorker) readFromStorage(ctx context.Context, key string) ([]byte, error) {
	reader, _, err := h.storage.GetFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get file from storage: %w", err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(io.LimitReader(reader, int64(maxImageSize)+1))
	if err != nil {
		return nil, fmt.Errorf("read file from storage: %w", err)
	}
	if len(data) > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes", len(data))
	}
	return data, nil
}
