package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/google/uuid"
)

type GPTHandler struct {
	db        *database.DB
	gptClient *gpt.Client
	repo      *repository.Repository
}

func NewGPTHandler(db *database.DB, gptClient *gpt.Client) *GPTHandler {
	return &GPTHandler{
		db:        db,
		gptClient: gptClient,
		repo:      repository.New(db),
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

	if err := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusProcessing); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	result, err := h.gptClient.ProcessRequest(ctx, payload.TextQuery, payload.FileKeys)
	if err != nil {
		slog.Error("GPT processing failed", "request_id", payload.RequestID, "error", err)
		// Try to create fallback response from EKG analysis data
		fallbackContent, fallbackErr := h.createFallbackResponse(ctx, payload)
		if fallbackErr == nil && fallbackContent != "" {
			slog.Info("created fallback response from EKG analysis", "request_id", payload.RequestID)
			result = &gpt.ProcessResult{
				Content:          fallbackContent,
				Model:            "fallback_ekg_analysis",
				TokensUsed:       0,
				ProcessingTimeMs: 0,
			}
		} else {
			if updateErr := h.repo.UpdateRequestStatus(ctx, payload.RequestID, models.StatusFailed); updateErr != nil {
				slog.Error("failed to update request status to failed", "request_id", payload.RequestID, "error", updateErr)
			}
			return fmt.Errorf("GPT processing failed: %w", err)
		}
	}

	if gpt.IsRefusal(result.Content) {
		slog.Warn("GPT returned refusal message, using fallback response",
			"request_id", payload.RequestID,
			"response_preview", func() string {
				if len(result.Content) > 200 {
					return result.Content[:200] + "..."
				}
				return result.Content
			}())
		fallbackContent, fallbackErr := h.createFallbackResponse(ctx, payload)
		if fallbackErr == nil && fallbackContent != "" {
			slog.Info("replaced GPT refusal with fallback response",
				"request_id", payload.RequestID,
				"fallback_length", len(fallbackContent))
			result.Content = fallbackContent
			result.Model = result.Model + "_with_fallback"
		} else {
			slog.Error("failed to create fallback response",
				"request_id", payload.RequestID,
				"error", fallbackErr)
		}
	}

	// Save response and update status in a transaction
	err = h.db.WithTx(ctx, func(tx database.Tx) error {
		txRepo := h.repo.WithTx(tx)

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

		slog.Info("GPT job completed successfully",
			"request_id", payload.RequestID,
			"response_id", response.ID,
			"tokens_used", result.TokensUsed,
			"processing_time_ms", result.ProcessingTimeMs,
		)

		return nil
	})

	return err
}

// createFallbackResponse creates a response from EKG analysis data when GPT fails or refuses
func (h *GPTHandler) createFallbackResponse(ctx context.Context, payload gpt.GPTJobPayload) (string, error) {
	request, err := h.repo.GetRequestByID(ctx, payload.RequestID)
	if err != nil {
		return "", fmt.Errorf("failed to get request: %w", err)
	}

	textQuery := ""
	if request.TextQuery != nil {
		textQuery = *request.TextQuery
	}

	// Try to find related EKG response by searching user's recent requests.
	userRequests, err := h.repo.GetRequestsByUserID(ctx, request.UserID, 10, 0)
	if err == nil {
		// First pass: prefer the EKG response that references this exact GPT request
		for i := 0; i < len(userRequests); i++ {
			if userRequests[i].ID == payload.RequestID {
				continue
			}
			ekg := h.findEKGContent(ctx, userRequests[i].ID)
			if ekg != nil && ekg.GPTRequestID == payload.RequestID.String() {
				return formatEKGFallback(ekg, textQuery), nil
			}
		}

		// Second pass: use any recent EKG response
		for i := 0; i < len(userRequests); i++ {
			if userRequests[i].ID == payload.RequestID {
				continue
			}
			if ekg := h.findEKGContent(ctx, userRequests[i].ID); ekg != nil {
				return formatEKGFallback(ekg, textQuery), nil
			}
		}
	}

	return formatBasicFallback(textQuery), nil
}

// findEKGContent loads a request by ID and tries to parse its response as EKG content.
func (h *GPTHandler) findEKGContent(ctx context.Context, requestID uuid.UUID) *models.EKGResponseContent {
	fullReq, err := h.repo.GetRequestByID(ctx, requestID)
	if err != nil || fullReq.Response == nil {
		return nil
	}
	ekg, _ := models.ParseEKGContent(fullReq.Response.Content)
	return ekg
}

// formatEKGFallback formats fallback response from typed EKG data
func formatEKGFallback(ekg *models.EKGResponseContent, textQuery string) string {
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
