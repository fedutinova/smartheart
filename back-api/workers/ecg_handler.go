package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
	"github.com/fedutinova/smartheart/back-api/storage"
)

// ECGWorker processes EKG analysis jobs.
type ECGWorker struct {
	txb       database.TxBeginner
	queue     job.Queue
	storage   storage.Storage
	repo      repository.RequestRepo
	quotaRepo repository.QuotaRepo
	gptClient gpt.Processor
	// cvClient is the rhythm classifier. Nil disables rhythm inference — the
	// worker still produces a structured measurement result via GPT.
	cvClient cv.Client
	hub      *notify.Hub
}

func NewECGWorker(
	txb database.TxBeginner,
	queue job.Queue,
	storageService storage.Storage,
	repo repository.Store,
	gptClient gpt.Processor,
	cvClient cv.Client,
	hub *notify.Hub,
) *ECGWorker {
	return &ECGWorker{
		txb:       txb,
		queue:     queue,
		storage:   storageService,
		repo:      repo,
		quotaRepo: repo,
		gptClient: gptClient,
		cvClient:  cvClient,
		hub:       hub,
	}
}

func (h *ECGWorker) HandleECGJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeECGAnalyze {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload job.ECGJobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal EKG job payload: %w", err)
	}

	err := h.processEKG(ctx, j, &payload)
	if err != nil {
		h.handleEKGFailure(ctx, &payload)
	}
	return err
}

func (h *ECGWorker) handleEKGFailure(ctx context.Context, payload *job.ECGJobPayload) {
	// Refund the free analyses counter so failed analyses don't count.
	if decErr := h.quotaRepo.DecrementFreeAnalysesUsed(ctx, payload.UserID); decErr != nil {
		slog.WarnContext(ctx, "Failed to decrement free analyses used after EKG failure", "user_id", payload.UserID, "error", decErr)
	}
	// Mark request as failed and notify user.
	if payload.RequestID == uuid.Nil {
		return
	}
	if updErr := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updErr != nil {
		slog.ErrorContext(ctx, "Failed to update request status to failed", "request_id", payload.RequestID, "error", updErr)
	}
	h.hub.Notify(payload.UserID, notify.Event{
		Type:      "request_completed",
		RequestID: payload.RequestID,
		Status:    models.StatusFailed,
	})
}

func (h *ECGWorker) processEKG(ctx context.Context, j *job.Job, payload *job.ECGJobPayload) error {
	slog.InfoContext(ctx, "Starting EKG analysis", "job_id", j.ID, "user_id", payload.UserID)

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

	if payload.ImageFileKey == "" {
		return fmt.Errorf("image_file_key is required")
	}
	imageData, err := h.readFromStorage(ctx, payload.ImageFileKey)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get EKG image", "job_id", j.ID, "error", err)
		return fmt.Errorf("failed to get image: %w", err)
	}
	imageKey := payload.ImageFileKey

	// Run GPT measurement extraction and CV rhythm classification in parallel.
	// GPT failure is fatal (no structured result → nothing to save). CV failure
	// is logged and degraded to a nil rhythm block — the user still gets the
	// measurements/indices view.
	systemPrompt, userPrompt := gpt.BuildECGMeasurementPrompt(payload.PaperSpeedMMS)

	var (
		gptResult    *gpt.ProcessResult
		rhythmResult *models.ECGRhythmResult
	)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		res, gerr := h.gptClient.ProcessStructuredECG(egCtx, []string{imageKey}, systemPrompt, userPrompt)
		if gerr != nil {
			slog.ErrorContext(egCtx, "GPT structured ECG call failed", "job_id", j.ID, "error", gerr)
			return fmt.Errorf("gpt analysis failed: %w", gerr)
		}
		gptResult = res
		return nil
	})
	if h.cvClient != nil {
		eg.Go(func() error {
			filename := fmt.Sprintf("ekg_%s.jpg", j.ID.String()[:8])
			pred, cerr := h.cvClient.Predict(
				egCtx,
				imageData,
				"image/jpeg",
				filename,
				payload.LayoutLabel,
				payload.PreprocessName,
			)
			if cerr != nil {
				slog.WarnContext(egCtx, "CV rhythm inference failed; degrading to nil rhythm block",
					"job_id", j.ID, "error", cerr)
				return nil
			}
			// CV succeeded — turn the prediction into a medical-grade text
			// conclusion via vision LLM. Soft-failure: if the LLM call fails,
			// we still publish the rhythm code without a narrative block.
			explanation, lerr := buildRhythmExplanation(egCtx, h.gptClient, imageKey, filename, payload.LayoutLabel, pred)
			if lerr != nil {
				slog.WarnContext(egCtx, "Rhythm explanation generation failed; publishing rhythm without explanation",
					"job_id", j.ID, "error", lerr)
			}
			rhythmResult = rhythmFromCV(pred, explanation)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Parse GPT JSON response
	slog.DebugContext(ctx, "GPT response received", "job_id", j.ID, "content_len", len(gptResult.Content))
	rawMeasurements, err := gpt.ParseECGMeasurementJSON(gptResult.Content)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to parse GPT ECG JSON", "job_id", j.ID, "error", err)
		return fmt.Errorf("parse GPT response: %w", err)
	}
	if rawMeasurements != nil && len(rawMeasurements.Leads) == 0 {
		slog.WarnContext(ctx, "GPT returned no measurements", "job_id", j.ID)
	}
	if rhythmResult != nil && !hasEnoughECGSignalForRhythm(rawMeasurements) {
		slog.WarnContext(ctx, "Suppressing rhythm result because ECG signal was not detected",
			"job_id", j.ID,
			"request_id", payload.RequestID,
			"leads_detected", countMeasuredLeads(rawMeasurements))
		rhythmResult = nil
	}

	// Post-process: convert small squares to mm/ms
	msPerSq := 1000.0 / payload.PaperSpeedMMS
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
	ecgContent := &models.ECGResponseContent{
		AnalysisType:     models.ECGModelStructured,
		Notes:            payload.Notes,
		Timestamp:        timestamp,
		JobID:            j.ID.String(),
		StructuredResult: structured,
		RhythmResult:     rhythmResult,
	}
	responseJSON, err := ecgContent.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	requestID := payload.RequestID
	if err := h.txb.WithTx(ctx, func(tx database.Tx) error {
		txRepo := repository.NewTxScoped(tx)

		response := &models.Response{
			ID:               uuid.New(),
			RequestID:        requestID,
			Content:          responseJSON,
			Model:            models.ECGModelStructured,
			TokensUsed:       gptResult.TokensUsed,
			ProcessingTimeMs: gptResult.ProcessingTimeMs,
		}
		if err := txRepo.CreateResponse(ctx, response); err != nil {
			return fmt.Errorf("save response: %w", err)
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
func (*ECGWorker) Close() {
	slog.Debug("EKG worker closed")
}

const maxImageSize = 10 * 1024 * 1024

func (h *ECGWorker) readFromStorage(ctx context.Context, key string) ([]byte, error) {
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
