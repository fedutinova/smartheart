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
// and parses the answer plus citations. Logs request size and processing time.
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

	contextSize := len(ecgContext)
	questionSize := len(question)
	start := time.Now()

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

	elapsed := time.Since(start)
	slog.DebugContext(ctx, "ecg chat rag call completed",
		"context_bytes", contextSize,
		"question_bytes", questionSize,
		"response_bytes", len(respBytes),
		"elapsed_ms", elapsed.Milliseconds())

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

// buildECGContextBlock extracts a comprehensive textual summary of the patient's ECG
// including demographics, measurements, indices, axis, rhythm, and key findings.
// Used as the system context for the RAG pipeline. Falls back gracefully when fields are missing.
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

// extractECGSummary parses the response JSON and builds a comprehensive ECG summary
// for LLM context. Prioritizes text_summary if available, otherwise builds a structured
// summary from measurements, indices, axis, rhythm, and interpretation items.
// Returns an empty string if the response is not structured ECG data.
func extractECGSummary(content string) string {
	var parsed struct {
		AnalysisType   string `json:"analysis_type"`
		StructuredData *struct {
			Measurements map[string]*float64 `json:"measurements"`
			Indices      *struct {
				SokolowLyon     *float64 `json:"sokolow_lyon_mV"`
				CornellVoltage  *float64 `json:"cornell_voltage_mV"`
				PegueroLoPresti *float64 `json:"peguero_lo_presti_mV"`
				Gubner          *float64 `json:"gubner_mV"`
				Lewis           *float64 `json:"lewis_mV"`
			} `json:"indices"`
			RVH *struct {
				RV1mV      *float64 `json:"RV1_mV"`
				ROverSV1   *float64 `json:"R_over_S_V1"`
				RV1PlusSV5 *float64 `json:"RV1_plus_SV5_mV"`
				RV1PlusSV6 *float64 `json:"RV1_plus_SV6_mV"`
			} `json:"rvh"`
			Axis *struct {
				AxisDeg        *float64 `json:"axis_deg"`
				Classification string   `json:"classification"`
			} `json:"axis_qrs"`
			Rhythm *struct {
				HRBpm *float64 `json:"HR_bpm"`
				QRSMs *float64 `json:"QRS_ms"`
				RRms  *float64 `json:"RR_ms"`
			} `json:"rhythm"`
			Transition *string `json:"transition_zone_lead"`
			Interpretation *struct {
				Items       []struct {
					Label     string `json:"label"`
					Value     string `json:"value"`
					Threshold string `json:"threshold"`
					Status    string `json:"status"`
					Group     string `json:"group"`
				} `json:"items"`
				TextSummary string `json:"text_summary"`
			} `json:"interpretation"`
		} `json:"structured_result"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return ""
	}
	if parsed.StructuredData == nil {
		return ""
	}

	sd := parsed.StructuredData
	if sd.Interpretation != nil && sd.Interpretation.TextSummary != "" {
		return sd.Interpretation.TextSummary
	}

	var parts []string

	if sd.Rhythm != nil {
		if hr := sd.Rhythm.HRBpm; hr != nil {
			parts = append(parts, fmt.Sprintf("ЧСС %.0f уд/мин", *hr))
		}
		if rr := sd.Rhythm.RRms; rr != nil {
			parts = append(parts, fmt.Sprintf("RR %.0f мс", *rr))
		}
		if qrs := sd.Rhythm.QRSMs; qrs != nil {
			parts = append(parts, fmt.Sprintf("QRS %.0f мс", *qrs))
		}
	}

	if sd.Axis != nil && sd.Axis.AxisDeg != nil {
		parts = append(parts, fmt.Sprintf("Ось QRS: %.0f°", *sd.Axis.AxisDeg))
		if sd.Axis.Classification != "" {
			parts = append(parts, fmt.Sprintf("Классификация оси: %s", sd.Axis.Classification))
		}
	}

	if sd.Indices != nil {
		var idxParts []string
		if sl := sd.Indices.SokolowLyon; sl != nil {
			idxParts = append(idxParts, fmt.Sprintf("Соколов-Лайон: %.1f мВ", *sl))
		}
		if cv := sd.Indices.CornellVoltage; cv != nil {
			idxParts = append(idxParts, fmt.Sprintf("Cornell: %.1f мВ", *cv))
		}
		if plp := sd.Indices.PegueroLoPresti; plp != nil {
			idxParts = append(idxParts, fmt.Sprintf("Peguero-Lo-Presti: %.1f мВ", *plp))
		}
		if idxParts != nil {
			parts = append(parts, "Индексы ГЛЖ: "+strings.Join(idxParts, "; "))
		}
	}

	if sd.RVH != nil {
		var rvhParts []string
		if rv1 := sd.RVH.RV1mV; rv1 != nil {
			rvhParts = append(rvhParts, fmt.Sprintf("RV1: %.1f мВ", *rv1))
		}
		if ros := sd.RVH.ROverSV1; ros != nil {
			rvhParts = append(rvhParts, fmt.Sprintf("R/S V1: %.2f", *ros))
		}
		if rvhParts != nil {
			parts = append(parts, "Маркеры ГПЖ: "+strings.Join(rvhParts, "; "))
		}
	}

	if sd.Measurements != nil {
		var measParts []string
		leads := []string{"RII", "SIII", "RaVL", "RV5", "RV6", "SV1", "SV2"}
		for _, lead := range leads {
			if v, ok := sd.Measurements[lead]; ok && v != nil {
				measParts = append(measParts, fmt.Sprintf("%s: %.1f мВ", lead, *v))
			}
		}
		if measParts != nil {
			parts = append(parts, "Ключевые отведения: "+strings.Join(measParts, "; "))
		}
	}

	if sd.Interpretation != nil {
		for _, item := range sd.Interpretation.Items {
			if item.Status == "positive" || item.Status == "abnormal" {
				if item.Threshold != "" {
					parts = append(parts, fmt.Sprintf("%s: %s (пороговое: %s)", item.Label, item.Value, item.Threshold))
				} else {
					parts = append(parts, fmt.Sprintf("%s: %s", item.Label, item.Value))
				}
			}
		}
	}

	if sd.Transition != nil && *sd.Transition != "" {
		parts = append(parts, fmt.Sprintf("Зона переходности: %s", *sd.Transition))
	}

	return strings.Join(parts, "; ")
}
