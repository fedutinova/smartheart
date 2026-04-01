package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/notify"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// GPTWorker processes GPT analysis jobs.
// Named differently from handler.GPTHandler to avoid confusion.
type GPTWorker struct {
	txb       database.TxBeginner
	gptClient gpt.Processor
	repo      repository.RequestRepo
	hub       *notify.Hub
}

func NewGPTWorker(txb database.TxBeginner, gptClient gpt.Processor, repo repository.RequestRepo, hub *notify.Hub) *GPTWorker {
	return &GPTWorker{
		txb:       txb,
		gptClient: gptClient,
		repo:      repo,
		hub:       hub,
	}
}

func (h *GPTWorker) HandleGPTJob(ctx context.Context, j *job.Job) error {
	if j.Type != job.TypeGPTProcess {
		return fmt.Errorf("unexpected job type: %s", j.Type)
	}

	var payload gpt.JobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	if err := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusProcessing); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	result, err := h.processWithFallback(ctx, payload)
	if err != nil {
		if updateErr := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updateErr != nil {
			slog.ErrorContext(ctx, "Failed to update request status to failed", "request_id", payload.RequestID, "error", updateErr)
		}
		h.notifyUser(payload.UserID, payload.RequestID, models.StatusFailed)
		return fmt.Errorf("gpt processing failed: %w", err)
	}

	if txErr := h.saveGPTResult(ctx, payload, result); txErr != nil {
		if updateErr := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updateErr != nil {
			slog.ErrorContext(ctx, "Failed to update request status to failed after tx error",
				"request_id", payload.RequestID, "error", updateErr)
		}
		h.notifyUser(payload.UserID, payload.RequestID, models.StatusFailed)
		return txErr
	}

	h.notifyUser(payload.UserID, payload.RequestID, models.StatusCompleted)
	return nil
}

// saveGPTResult persists the GPT response and marks the request as completed in a single transaction.
func (h *GPTWorker) saveGPTResult(ctx context.Context, payload gpt.JobPayload, result *gpt.ProcessResult) error {
	return h.txb.WithTx(ctx, func(tx database.Tx) error {
		txRepo := repository.NewTxScoped(tx)

		response := &models.Response{
			RequestID:        payload.RequestID,
			Content:          result.Content,
			Model:            result.Model,
			TokensUsed:       result.TokensUsed,
			ProcessingTimeMs: result.ProcessingTimeMs,
		}
		if err := txRepo.CreateResponse(ctx, response); err != nil {
			return fmt.Errorf("failed to save response: %w", err)
		}

		if err := txRepo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusCompleted); err != nil {
			return fmt.Errorf("failed to update request status: %w", err)
		}

		slog.InfoContext(ctx, "GPT job completed successfully",
			"request_id", payload.RequestID,
			"response_id", response.ID,
			"tokens_used", result.TokensUsed,
			"processing_time_ms", result.ProcessingTimeMs,
		)

		return nil
	})
}

func (h *GPTWorker) notifyUser(userID, requestID uuid.UUID, status string) {
	if h.hub == nil {
		return
	}
	h.hub.Notify(userID, notify.Event{
		Type:      "request_" + status,
		RequestID: requestID,
		Status:    status,
	})
}

// processWithFallback calls GPT and falls back to EKG data if GPT fails or refuses.
func (h *GPTWorker) processWithFallback(ctx context.Context, payload gpt.JobPayload) (*gpt.ProcessResult, error) {
	result, gptErr := h.gptClient.ProcessRequest(ctx, payload.TextQuery, payload.FileKeys)

	// Happy path: GPT succeeded and didn't refuse
	if gptErr == nil && result != nil && !gpt.IsRefusal(result.Content) {
		return result, nil
	}

	// Guard against nil result with nil error (shouldn't happen but prevents panic)
	if gptErr == nil && result == nil {
		gptErr = errors.New("gpt returned nil result without error")
	}

	// Only attempt EKG fallback for EKG-originated GPT requests.
	if !isECGRequest(payload.TextQuery) {
		if gptErr != nil {
			return nil, gptErr
		}
		return result, nil
	}

	// Try fallback
	if gptErr != nil {
		slog.WarnContext(ctx, "GPT failed, attempting EKG fallback", "request_id", payload.RequestID, "error", gptErr)
	} else {
		slog.WarnContext(ctx, "GPT returned refusal, attempting EKG fallback",
			"request_id", payload.RequestID,
			"response_preview", truncate(result.Content, 200))
	}

	fallbackContent, fallbackErr := h.createFallbackResponse(ctx, payload)
	if fallbackErr != nil || fallbackContent == "" {
		if gptErr != nil {
			return nil, gptErr
		}
		// Refusal with no fallback — return original response as-is
		return result, nil
	}

	if gptErr != nil {
		// Complete failure — use fallback as the entire result
		return &gpt.ProcessResult{ //nolint:nilerr // intentionally return fallback on GPT error
			Content: fallbackContent,
			Model:   "fallback_ekg_analysis",
		}, nil
	}

	// Refusal — replace content but keep metadata
	slog.InfoContext(ctx, "Replaced GPT refusal with fallback response",
		"request_id", payload.RequestID,
		"fallback_length", len(fallbackContent))
	result.Content = fallbackContent
	result.Model += "_with_fallback"
	return result, nil
}

// createFallbackResponse creates a response from EKG analysis data when GPT fails or refuses
func (h *GPTWorker) createFallbackResponse(ctx context.Context, payload gpt.JobPayload) (string, error) {
	request, err := h.repo.GetRequestByID(ctx, payload.RequestID)
	if err != nil {
		return "", fmt.Errorf("failed to get request: %w", err)
	}

	textQuery := ""
	if request.TextQuery != nil {
		textQuery = *request.TextQuery
	}

	// Fetch recent requests with responses in a single query (avoids N+1).
	userRequests, err := h.repo.GetRecentRequestsWithResponses(ctx, request.UserID, 10)
	if err != nil {
		return formatBasicFallback(textQuery), nil //nolint:nilerr // intentionally return fallback on fetch error
	}

	// First pass: prefer the EKG response that references this exact GPT request
	for i := range userRequests {
		if userRequests[i].ID == payload.RequestID || userRequests[i].Response == nil {
			continue
		}
		if ekg, _ := models.ParseECGContent(userRequests[i].Response.Content); ekg != nil && ekg.GPTRequestID == payload.RequestID.String() {
			return formatECGFallback(ekg, textQuery), nil
		}
	}

	// Second pass: use any recent EKG response
	for i := range userRequests {
		if userRequests[i].ID == payload.RequestID || userRequests[i].Response == nil {
			continue
		}
		if ekg, _ := models.ParseECGContent(userRequests[i].Response.Content); ekg != nil {
			return formatECGFallback(ekg, textQuery), nil
		}
	}

	return formatBasicFallback(textQuery), nil
}

// formatECGFallback formats fallback response from typed EKG data
func formatECGFallback(ekg *models.ECGResponseContent, textQuery string) string {
	result := "Автоматический анализ ЭКГ изображения временно недоступен.\n\n"
	result += fmt.Sprintf("Время анализа: %s\n", ekg.Timestamp)

	if ekg.Notes != "" {
		result += fmt.Sprintf("\nПримечания пользователя:\n%s\n", ekg.Notes)
	}
	if textQuery != "" && textQuery != ekg.Notes {
		result += fmt.Sprintf("\nДополнительная информация:\n%s\n", textQuery)
	}

	result += "\nПримечание: GPT-интерпретация недоступна. Пожалуйста, попробуйте повторить запрос позже."
	return result
}

// formatBasicFallback creates a basic fallback response
func formatBasicFallback(textQuery string) string {
	result := "Автоматический анализ изображения временно недоступен.\n\n"

	if textQuery != "" {
		result += fmt.Sprintf("Контекст запроса:\n%s\n\n", textQuery)
	}

	result += "Пожалуйста, попробуйте повторить запрос позже."
	return result
}

// isECGRequest checks whether the text query originated from the EKG analysis
// pipeline rather than from a direct user GPT request.
func isECGRequest(textQuery string) bool {
	return strings.Contains(textQuery, "Analyze this ECG/EKG image")
}

// truncate returns the first n runes of s, appending "..." if truncated.
// Operates on runes to avoid splitting multi-byte UTF-8 characters.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
