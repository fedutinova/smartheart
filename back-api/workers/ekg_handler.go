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

	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/storage"
	"github.com/fedutinova/smartheart/back-api/validation"
	"github.com/google/uuid"
)

// EKGWorker processes EKG analysis jobs.
// Named differently from handler.EKGHandler to avoid confusion.
type EKGWorker struct {
	txb     database.TxBeginner
	queue   job.Queue
	storage storage.Storage
	repo    repository.RequestRepo
}

func NewEKGWorker(txb database.TxBeginner, queue job.Queue, storageService storage.Storage, repo repository.RequestRepo) *EKGWorker {
	return &EKGWorker{
		txb:     txb,
		queue:   queue,
		storage: storageService,
		repo:    repo,
	}
}

func (h *EKGWorker) HandleEKGJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeEKGAnalyze {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload job.EKGJobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal EKG job payload: %w", err)
	}

	slog.Info("starting EKG analysis",
		"job_id", j.ID,
		"user_id", payload.UserID,
		"mode", ekgJobMode(payload))

	var imageData []byte
	var err error

	if payload.ImageFileKey != "" {
		// File mode: image already in storage
		imageData, err = h.readFromStorage(ctx, payload.ImageFileKey)
	} else {
		// URL mode: download from external URL
		imageData, err = h.downloadImage(ctx, payload.ImageTempURL)
	}
	if err != nil {
		slog.Error("failed to get EKG image", "job_id", j.ID, "error", err)
		return fmt.Errorf("failed to get image: %w", err)
	}

	// Save results and trigger GPT analysis directly — no OpenCV preprocessing
	if err := h.saveEKGResults(ctx, j.ID, payload, imageData); err != nil {
		return fmt.Errorf("failed to save EKG results: %w", err)
	}

	slog.Info("ekg analysis job completed, GPT analysis triggered",
		"job_id", j.ID)

	return nil
}

// validateImageURL performs basic scheme/hostname checks before downloading.
func validateImageURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty image URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http/https allowed)", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "0.0.0.0" {
		return fmt.Errorf("requests to localhost are not allowed")
	}

	return nil
}

// isPrivateIP reports whether ip is loopback, private, link-local, or unspecified.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// sharedSSRFTransport is a package-level transport reused across all image
// downloads to avoid allocating a new transport per request.
var sharedSSRFTransport = newSSRFSafeTransport()

// newSSRFSafeTransport returns an http.Transport that rejects connections to
// private/reserved IP addresses at dial time, preventing TOCTOU SSRF.
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

			// Connect to the first allowed IP
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
		DisableKeepAlives: true,
	}
}

// downloadImage downloads image from URL with SSRF protection, timeout and size limits.
func (h *EKGWorker) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	if err := validateImageURL(imageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: sharedSSRFTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
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

	imageData, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("image too large after download: %d bytes (max %d)", len(imageData), maxImageSize)
	}

	slog.Debug("downloaded EKG image",
		"url", imageURL,
		"content_type", contentType,
		"size_bytes", len(imageData))

	return imageData, nil
}

// isValidImageContentType checks if the content type is a valid image or PDF format
func isValidImageContentType(contentType string) bool {
	return validation.IsImageType(contentType) || contentType == "application/pdf"
}

func ekgJobMode(p job.EKGJobPayload) string {
	if p.ImageFileKey != "" {
		return "file"
	}
	return "url"
}

const maxImageSize = 10 * 1024 * 1024 // 10MB

// readFromStorage reads an already-uploaded image from storage.
func (h *EKGWorker) readFromStorage(ctx context.Context, key string) ([]byte, error) {
	reader, _, err := h.storage.GetFile(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get file from storage: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, int64(maxImageSize)+1))
	if err != nil {
		return nil, fmt.Errorf("read file from storage: %w", err)
	}
	if len(data) > maxImageSize {
		return nil, fmt.Errorf("image too large: %d bytes (max %d)", len(data), maxImageSize)
	}

	slog.Debug("read EKG image from storage", "key", key, "size_bytes", len(data))
	return data, nil
}

func (h *EKGWorker) saveEKGResults(ctx context.Context, jobID uuid.UUID, payload job.EKGJobPayload, imageData []byte) error {
	requestID := payload.RequestID
	if requestID == uuid.Nil {
		requestID = uuid.New()
	}

	// Upload image to storage first (outside transaction — idempotent).
	gptPrep, err := h.prepareGPTAnalysis(ctx, jobID, payload, imageData)
	if err != nil {
		slog.Warn("failed to prepare GPT analysis", "error", err, "job_id", jobID)
		// Continue without GPT — EKG result is still saved.
	}

	gptRequestID := uuid.Nil
	if gptPrep != nil {
		gptRequestID = gptPrep.requestID
	}

	responseJSON, err := h.buildEKGResponseJSON(jobID, gptRequestID, payload.Notes)
	if err != nil {
		return fmt.Errorf("failed to marshal response content: %w", err)
	}

	// All DB writes in a single transaction to avoid orphaned records.
	if err := h.persistEKGResults(ctx, jobID, requestID, payload, responseJSON, gptPrep); err != nil {
		return err
	}

	// Enqueue GPT job after commit — if this fails the GPT request stays
	// as "pending" in DB (recoverable) but we don't have orphaned records.
	if gptPrep != nil {
		if gptJobID, err := h.enqueueGPTJob(ctx, gptPrep); err != nil {
			slog.Error("failed to enqueue GPT job after EKG commit",
				"error", err, "gpt_request_id", gptPrep.requestID)
		} else {
			slog.Info("gpt analysis job triggered for EKG image",
				"ekg_job_id", jobID,
				"ekg_request_id", requestID,
				"gpt_job_id", gptJobID,
				"gpt_request_id", gptPrep.requestID,
				"image_key", gptPrep.uploadKey)
		}
	}

	return nil
}

// gptAnalysisPrep holds pre-computed data for GPT analysis,
// ready to be persisted inside a transaction.
type gptAnalysisPrep struct {
	requestID uuid.UUID
	textQuery string
	uploadKey string
	uploadURL string
	filename  string
	fileSize  int64
	userID    uuid.UUID
}

// prepareGPTAnalysis uploads the image (or reuses existing key) and prepares
// data for the GPT request/file records without writing to the DB yet.
func (h *EKGWorker) prepareGPTAnalysis(ctx context.Context, ekgJobID uuid.UUID, payload job.EKGJobPayload, imageData []byte) (*gptAnalysisPrep, error) {
	filename := fmt.Sprintf("ekg_%s.jpg", ekgJobID.String()[:8])

	var uploadKey, uploadURL string
	if payload.ImageFileKey != "" {
		// File mode: image already in storage, reuse existing key
		uploadKey = payload.ImageFileKey
	} else {
		// URL mode: upload downloaded image to storage
		uploadResult, err := h.storage.UploadFile(ctx, filename, bytes.NewReader(imageData), "image/jpeg")
		if err != nil {
			return nil, fmt.Errorf("failed to upload image: %w", err)
		}
		uploadKey = uploadResult.Key
		uploadURL = uploadResult.URL
	}

	return &gptAnalysisPrep{
		requestID: uuid.New(),
		textQuery: buildEKGPrompt(payload.Notes),
		uploadKey: uploadKey,
		uploadURL: uploadURL,
		filename:  filename,
		fileSize:  int64(len(imageData)),
		userID:    payload.UserID,
	}, nil
}

// buildEKGResponseJSON creates the JSON content for an EKG response.
func (h *EKGWorker) buildEKGResponseJSON(jobID, gptRequestID uuid.UUID, notes string) (string, error) {
	ekgContent := &models.EKGResponseContent{
		AnalysisType: models.EKGModelDirect,
		Notes:        notes,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		JobID:        jobID.String(),
	}
	if gptRequestID != uuid.Nil {
		ekgContent.GPTRequestID = gptRequestID.String()
		ekgContent.GPTInterpretationStatus = "pending"
	}
	return ekgContent.Marshal()
}

// persistEKGResults saves EKG response, GPT request/file, and status updates
// in a single transaction to avoid orphaned records.
func (h *EKGWorker) persistEKGResults(ctx context.Context, jobID, requestID uuid.UUID, payload job.EKGJobPayload, responseJSON string, gptPrep *gptAnalysisPrep) error {
	return h.txb.WithTx(ctx, func(tx database.Tx) error {
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

		// Create GPT request and file inside the same transaction.
		if gptPrep != nil {
			gptRequest := &models.Request{
				ID:        gptPrep.requestID,
				UserID:    gptPrep.userID,
				TextQuery: &gptPrep.textQuery,
				Status:    models.StatusPending,
			}
			if err := txRepo.CreateRequest(ctx, gptRequest); err != nil {
				return fmt.Errorf("failed to create GPT request: %w", err)
			}

			fileModel := &models.File{
				ID:               uuid.New(),
				RequestID:        gptPrep.requestID,
				OriginalFilename: gptPrep.filename,
				FileType:         "image/jpeg",
				FileSize:         gptPrep.fileSize,
				S3Key:            gptPrep.uploadKey,
				S3URL:            gptPrep.uploadURL,
			}
			if err := txRepo.CreateFile(ctx, fileModel); err != nil {
				return fmt.Errorf("failed to create file record: %w", err)
			}
		}

		slog.Info("saved EKG analysis results",
			"job_id", jobID,
			"request_id", requestID,
			"response_id", response.ID)

		return nil
	})
}

// enqueueGPTJob enqueues a GPT processing job from prepared data.
func (h *EKGWorker) enqueueGPTJob(ctx context.Context, prep *gptAnalysisPrep) (uuid.UUID, error) {
	gptPayload := gpt.JobPayload{
		RequestID: prep.requestID,
		TextQuery: prep.textQuery,
		FileKeys:  []string{prep.uploadKey},
		UserID:    prep.userID,
	}

	payloadBytes, err := json.Marshal(gptPayload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal GPT payload: %w", err)
	}

	return h.queue.Enqueue(ctx, &job.Job{
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	})
}

// Close cleans up resources used by the EKG worker.
func (h *EKGWorker) Close() {
	slog.Debug("ekg worker closed")
}

const ekgPromptTemplate = `Analyze this ECG/EKG image. Describe what you observe in Russian language.

Structure your analysis:
1. Качество изображения: четкость, наличие артефактов, видимость отведений
2. Ритм: регулярный/нерегулярный, приблизительная ЧСС если видна разметка
3. Зубцы и интервалы: P, QRS, T — форма, амплитуда, длительность (если видна калибровка)
4. Сегменты: ST-сегмент (элевация/депрессия), PR-интервал, QT-интервал
5. Особенности: любые отклонения от нормального синусового ритма
`

const ekgPromptDisclaimer = `
This is for educational and technical analysis. Describe observations without making diagnostic conclusions.`

func buildEKGPrompt(userNotes string) string {
	if userNotes != "" {
		return ekgPromptTemplate + "\nAdditional context from user:\n" + userNotes + "\n" + ekgPromptDisclaimer
	}
	return ekgPromptTemplate + ekgPromptDisclaimer
}
