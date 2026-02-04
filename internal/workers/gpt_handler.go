package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
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

	// Check if GPT returned a refusal message
	refusalPatterns := []string{
		"i'm sorry",
		"i cannot",
		"can't assist",
		"unable to",
		"not able",
		"не могу",
		"извините",
		"не в состоянии",
		"cannot analyze",
		"cannot recognize",
	}
	isRefusal := false
	contentLower := strings.ToLower(result.Content)
	for _, pattern := range refusalPatterns {
		if strings.Contains(contentLower, pattern) {
			isRefusal = true
			break
		}
	}

	if isRefusal {
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
	// Get the GPT request to extract text_query with EKG data
	request, err := h.repo.GetRequestByID(ctx, payload.RequestID)
	if err != nil {
		return "", fmt.Errorf("failed to get request: %w", err)
	}

	textQuery := ""
	if request.TextQuery != nil {
		textQuery = *request.TextQuery
	}

	// Try to find related EKG response by searching user's requests
	// EKG response contains gpt_request_id that matches payload.RequestID
	userRequests, err := h.repo.GetRequestsByUserID(ctx, request.UserID)
	if err == nil {
		// Look for EKG response that references this GPT request
		for i := len(userRequests) - 1; i >= 0; i-- {
			if userRequests[i].Response != nil && userRequests[i].ID != payload.RequestID {
				var ekgData map[string]interface{}
				if err := json.Unmarshal([]byte(userRequests[i].Response.Content), &ekgData); err == nil {
					if analysisType, ok := ekgData["analysis_type"].(string); ok && analysisType == "ekg_preprocessing" {
						// Check if this EKG response references our GPT request
						if gptReqID, ok := ekgData["gpt_request_id"].(string); ok && gptReqID == payload.RequestID.String() {
							// Found matching EKG analysis data
							return h.formatFallbackFromEKGData(ekgData, textQuery), nil
						}
					}
				}
			}
		}

		// Fallback: look for most recent EKG response for this user (within last 10 requests)
		for i := len(userRequests) - 1; i >= 0 && i >= len(userRequests)-10; i-- {
			if userRequests[i].Response != nil && userRequests[i].ID != payload.RequestID {
				var ekgData map[string]interface{}
				if err := json.Unmarshal([]byte(userRequests[i].Response.Content), &ekgData); err == nil {
					if analysisType, ok := ekgData["analysis_type"].(string); ok && analysisType == "ekg_preprocessing" {
						// Found EKG analysis data (most recent)
						return h.formatFallbackFromEKGData(ekgData, textQuery), nil
					}
				}
			}
		}
	}

	// If no EKG data found, create basic response from text_query
	return h.formatBasicFallback(textQuery), nil
}

// formatFallbackFromEKGData formats fallback response from EKG analysis data
func (h *GPTHandler) formatFallbackFromEKGData(ekgData map[string]interface{}, textQuery string) string {
	result := "Анализ выполнен на основе технической обработки изображения:\n\n"

	if signalLength, ok := ekgData["signal_length"].(float64); ok {
		result += fmt.Sprintf("1. Длина сигнала: %.2f пикселей\n", signalLength)
	}

	if features, ok := ekgData["signal_features"].(map[string]interface{}); ok {
		result += "2. Технические характеристики сигнала:\n"
		if width, ok := features["signal_width"].(float64); ok {
			result += fmt.Sprintf("   - Ширина сигнала: %.0f пикселей\n", width)
		}
		if amplitude, ok := features["amplitude_range"].(float64); ok {
			result += fmt.Sprintf("   - Диапазон амплитуды: %.0f пикселей\n", amplitude)
		}
		if baseline, ok := features["baseline"].(float64); ok {
			result += fmt.Sprintf("   - Базовая линия: %.2f\n", baseline)
		}
		if stdDev, ok := features["standard_deviation"].(float64); ok {
			result += fmt.Sprintf("   - Стандартное отклонение: %.2f\n", stdDev)
		}
		if points, ok := features["points_count"].(float64); ok {
			result += fmt.Sprintf("   - Количество точек контура: %.0f\n", points)
		}
	}

	if steps, ok := ekgData["processing_steps"].([]interface{}); ok {
		result += "\n3. Этапы обработки изображения:\n"
		for i, step := range steps {
			if stepStr, ok := step.(string); ok {
				stepNames := map[string]string{
					"resized":                 "Изменение размера",
					"grayscale":               "Перевод в градации серого",
					"contrast_enhanced":       "Улучшение контраста",
					"binarized":               "Бинаризация",
					"morphological_processed": "Морфологическая обработка",
					"signal_extracted":        "Извлечение сигнала",
				}
				if name, exists := stepNames[stepStr]; exists {
					result += fmt.Sprintf("   %d. %s\n", i+1, name)
				} else {
					result += fmt.Sprintf("   %d. %s\n", i+1, stepStr)
				}
			}
		}
	}

	if textQuery != "" {
		result += fmt.Sprintf("\n4. Дополнительная информация:\n%s\n", textQuery)
	}

	result += "\nПримечание: Полный анализ изображения недоступен. Представлены результаты технической обработки."

	return result
}

// formatBasicFallback creates a basic fallback response
func (h *GPTHandler) formatBasicFallback(textQuery string) string {
	result := "Анализ изображения выполнен техническими средствами.\n\n"

	if textQuery != "" {
		result += fmt.Sprintf("Данные предобработки:\n%s\n\n", textQuery)
	}

	result += "Полный анализ изображения недоступен. Результаты основаны на технической обработке данных."

	return result
}
