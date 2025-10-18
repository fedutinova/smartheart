package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nuromirg/smartheart/internal/database"
	"github.com/nuromirg/smartheart/internal/gpt"
	"github.com/nuromirg/smartheart/internal/job"
	"github.com/nuromirg/smartheart/internal/models"
)

type GPTHandler struct {
	db        *database.DB
	gptClient *gpt.Client
}

func NewGPTHandler(db *database.DB, gptClient *gpt.Client) *GPTHandler {
	return &GPTHandler{
		db:        db,
		gptClient: gptClient,
	}
}

func (h *GPTHandler) HandleGPTJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeGPTProcess {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload gpt.GPTJobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	if err := h.updateRequestStatus(ctx, payload.RequestID, models.StatusProcessing); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	result, err := h.gptClient.ProcessRequest(ctx, payload.TextQuery, payload.FileKeys)
	if err != nil {
		slog.Error("GPT processing failed", "request_id", payload.RequestID, "error", err)
		if updateErr := h.updateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updateErr != nil {
			slog.Error("failed to update request status to failed", "request_id", payload.RequestID, "error", updateErr)
		}
		return fmt.Errorf("GPT processing failed: %w", err)
	}

	responseID, err := h.saveResponse(ctx, payload.RequestID, result)
	if err != nil {
		return fmt.Errorf("failed to save response: %w", err)
	}

	if err := h.updateRequestStatus(ctx, payload.RequestID, models.StatusCompleted); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	slog.Info("GPT job completed successfully", 
		"request_id", payload.RequestID, 
		"response_id", responseID,
		"tokens_used", result.TokensUsed,
		"processing_time_ms", result.ProcessingTimeMs,
	)

	return nil
}

func (h *GPTHandler) updateRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error {
	query := `UPDATE requests SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := h.db.Pool().Exec(ctx, query, status, requestID)
	return err
}

func (h *GPTHandler) saveResponse(ctx context.Context, requestID uuid.UUID, result *gpt.ProcessResult) (uuid.UUID, error) {
	responseID := uuid.New()
	query := `
		INSERT INTO responses (id, request_id, content, model, tokens_used, processing_time_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	
	_, err := h.db.Pool().Exec(ctx, query,
		responseID,
		requestID,
		result.Content,
		result.Model,
		result.TokensUsed,
		result.ProcessingTimeMs,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return responseID, nil
}