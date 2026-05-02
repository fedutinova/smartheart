package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

const (
	ecgChatMaxQuestionLen = 2000
	ecgChatRAGTimeout     = 120 * time.Second
	ecgChatRAGNResults    = 5
)

// ECGChatService handles contextual chat anchored to a specific ECG analysis.
type ECGChatService interface {
	GetMessages(ctx context.Context, requestID, userID uuid.UUID) ([]models.ECGChatMessage, error)
	SendMessage(ctx context.Context, requestID, userID uuid.UUID, content string) (*models.ECGChatMessage, error)
}

type ecgChatService struct {
	repo   repository.Store
	ragURL string
	client *http.Client
}

// NewECGChatService creates a chat service that proxies questions to the RAG
// pipeline with the user's ECG result injected as context.
func NewECGChatService(repo repository.Store, ragURL string) ECGChatService {
	return &ecgChatService{
		repo:   repo,
		ragURL: ragURL,
		client: &http.Client{Timeout: ecgChatRAGTimeout},
	}
}

// GetMessages returns chat history for an ECG, scoped to the requesting user.
func (s *ecgChatService) GetMessages(ctx context.Context, requestID, userID uuid.UUID) ([]models.ECGChatMessage, error) {
	if err := s.assertOwnership(ctx, requestID, userID); err != nil {
		return nil, err
	}
	msgs, err := s.repo.GetECGChatMessages(ctx, requestID, userID)
	if err != nil {
		return nil, fmt.Errorf("ecg chat: get messages: %w", err)
	}
	return msgs, nil
}

// SendMessage asks the RAG service for an answer using the ECG result as
// context, then persists user and assistant messages atomically in a single
// transaction.
//
// The RAG call and the database writes use a context detached from the
// caller's request via context.WithoutCancel. Even if the HTTP client
// disconnects mid-flight (RAG can take 15-20s with gpt-4o), the answer is
// still computed and stored — the user will see the full exchange when
// they reload the chat history.
func (s *ecgChatService) SendMessage(ctx context.Context, requestID, userID uuid.UUID, content string) (*models.ECGChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("ecg chat: empty message: %w", apperr.ErrValidation)
	}
	if len(content) > ecgChatMaxQuestionLen {
		return nil, fmt.Errorf("ecg chat: message too long (max %d chars): %w",
			ecgChatMaxQuestionLen, apperr.ErrValidation)
	}

	req, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("ecg chat: load request: %w", err)
	}
	if req.UserID != userID {
		return nil, apperr.ErrRequestNotFound
	}

	contextBlock := buildECGContextBlock(req)

	// Detach from the caller's context so a disconnected client cannot
	// kill the RAG call or the subsequent transaction. A fresh timeout
	// keeps the work bounded.
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ecgChatRAGTimeout)
	defer cancel()

	answer, citations, err := s.askRAG(bgCtx, contextBlock, content)
	if err != nil {
		slog.ErrorContext(ctx, "ecg chat: rag call failed",
			"request_id", requestID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("ecg chat: ask rag: %w", err)
	}

	userMsg := &models.ECGChatMessage{
		RequestID: requestID,
		UserID:    userID,
		Role:      models.ECGChatRoleUser,
		Content:   content,
	}
	assistantMsg := &models.ECGChatMessage{
		RequestID: requestID,
		UserID:    userID,
		Role:      models.ECGChatRoleAssistant,
		Content:   answer,
		Citations: citations,
	}

	err = s.repo.RunTx(bgCtx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.CreateECGChatMessage(bgCtx, userMsg); err != nil {
			return fmt.Errorf("persist user message: %w", err)
		}
		if err := txRepo.CreateECGChatMessage(bgCtx, assistantMsg); err != nil {
			return fmt.Errorf("persist assistant message: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ecg chat: persist messages: %w", err)
	}
	return assistantMsg, nil
}

// assertOwnership verifies the request exists and belongs to the user.
func (s *ecgChatService) assertOwnership(ctx context.Context, requestID, userID uuid.UUID) error {
	req, err := s.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("ecg chat: load request for ownership check: %w", err)
	}
	if req.UserID != userID {
		return apperr.ErrRequestNotFound
	}
	return nil
}

type ragQuery struct {
	Question string `json:"question"`
	NResults int    `json:"n_results,omitempty"`
}

type ragSource struct {
	DocName    string  `json:"doc_name"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	Preview    string  `json:"preview"`
}

type ragResponse struct {
	Answer  string      `json:"answer"`
	Sources []ragSource `json:"sources"`
}

// askRAG sends the question (with ECG context prepended) to the RAG service
// and parses the answer plus citations.
func (s *ecgChatService) askRAG(ctx context.Context, ecgContext, question string) (string, []models.ECGChatCitation, error) {
	if s.ragURL == "" {
		return "", nil, fmt.Errorf("rag service not configured")
	}

	payload := ragQuery{
		Question: ecgContext + "\n\nВопрос пользователя: " + question,
		NResults: ecgChatRAGNResults,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("marshal rag query: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.ragURL+"/query", bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("build rag http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("call rag: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("rag returned status %d", resp.StatusCode)
	}
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", nil, fmt.Errorf("read rag response: %w", err)
	}

	var parsed ragResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", nil, fmt.Errorf("parse rag response: %w", err)
	}

	citations := make([]models.ECGChatCitation, 0, len(parsed.Sources))
	for _, src := range parsed.Sources {
		citations = append(citations, models.ECGChatCitation{
			Title:   src.DocName,
			Source:  fmt.Sprintf("Чанк %d (релевантность %.2f)", src.ChunkIndex, src.Score),
			Excerpt: src.Preview,
		})
	}
	return parsed.Answer, citations, nil
}

// buildECGContextBlock extracts a compact textual summary of the patient's ECG
// from the request + its latest response. Used as the system context for the
// chat assistant. Falls back gracefully when fields are missing.
func buildECGContextBlock(req *models.Request) string {
	var sb strings.Builder
	sb.WriteString("Контекст: пользователь обсуждает результаты своей ЭКГ.")

	if req.ECGAge != nil || req.ECGSex != nil {
		sb.WriteString(" Параметры пациента:")
		if req.ECGAge != nil {
			sb.WriteString(fmt.Sprintf(" возраст %d лет;", *req.ECGAge))
		}
		if req.ECGSex != nil {
			sex := *req.ECGSex
			switch sex {
			case "male":
				sex = "мужской"
			case "female":
				sex = "женский"
			}
			sb.WriteString(fmt.Sprintf(" пол %s;", sex))
		}
	}

	if req.Response != nil && req.Response.Content != "" {
		summary := extractECGSummary(req.Response.Content)
		if summary != "" {
			sb.WriteString("\nКлючевые находки ЭКГ: ")
			sb.WriteString(summary)
		}
	}
	return sb.String()
}

// extractECGSummary parses the response JSON and pulls out a short summary line.
// Returns an empty string if the response is not structured ECG data.
func extractECGSummary(content string) string {
	var parsed struct {
		AnalysisType   string `json:"analysis_type"`
		StructuredData *struct {
			Interpretation *struct {
				TextSummary string `json:"text_summary"`
				Summary     []struct {
					Label  string `json:"label"`
					Value  string `json:"value"`
					Status string `json:"status"`
				} `json:"summary"`
			} `json:"interpretation"`
			Rhythm *struct {
				HRBpm *float64 `json:"HR_bpm"`
				QRSMs *float64 `json:"QRS_ms"`
			} `json:"rhythm"`
		} `json:"structured_result"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return ""
	}
	if parsed.StructuredData == nil {
		return ""
	}

	if interp := parsed.StructuredData.Interpretation; interp != nil && interp.TextSummary != "" {
		return interp.TextSummary
	}

	var parts []string
	if parsed.StructuredData.Rhythm != nil {
		if hr := parsed.StructuredData.Rhythm.HRBpm; hr != nil {
			parts = append(parts, fmt.Sprintf("ЧСС %.0f уд/мин", *hr))
		}
		if qrs := parsed.StructuredData.Rhythm.QRSMs; qrs != nil {
			parts = append(parts, fmt.Sprintf("QRS %.0f мс", *qrs))
		}
	}
	if interp := parsed.StructuredData.Interpretation; interp != nil {
		for _, s := range interp.Summary {
			if s.Status == "positive" || s.Status == "abnormal" {
				parts = append(parts, fmt.Sprintf("%s: %s", s.Label, s.Value))
			}
		}
	}
	return strings.Join(parts, "; ")
}
