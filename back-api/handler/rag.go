package handler

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
	"github.com/sashabaranov/go-openai"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// RAGHandler proxies knowledge-base queries to the RAG microservice.
type RAGHandler struct {
	ragURL      string
	client      *http.Client
	repo        repository.Store
	judgeClient *openai.Client // nil when no API key configured
}

// NewRAGHandler creates a handler that forwards requests to the RAG service.
func NewRAGHandler(ragURL string, repo repository.Store, apiKey string) *RAGHandler {
	h := &RAGHandler{
		ragURL: ragURL,
		client: &http.Client{Timeout: 120 * time.Second},
		repo:   repo,
	}
	if apiKey != "" {
		h.judgeClient = openai.NewClient(apiKey)
	}
	return h
}

type ragQueryRequest struct {
	Question string `json:"question"            validate:"required,min=2,max=2000"`
	NResults int    `json:"n_results,omitempty" validate:"omitempty,gte=1,lte=20"`
}

type ragEmbedRequest struct {
	Text string `json:"text"`
}

type ragEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
	Model     string    `json:"model"`
	Dimension int       `json:"dimensions"`
}

// Query handles POST /v1/rag/query — validates input, proxies to RAG service,
// and records the request/response in the database for performance tracking.
func (h *RAGHandler) Query(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if h.ragURL == "" {
		writeError(w, http.StatusServiceUnavailable, "RAG service not configured")
		return
	}

	var req ragQueryRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	if req.NResults <= 0 {
		req.NResults = 5
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no auth context")
		return
	}

	body, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal request")
		return
	}

	// Create request record after marshal so we don't leave orphaned rows on marshal failure.
	requestID := uuid.New()
	question := req.Question
	dbReq := &models.Request{
		ID:        requestID,
		UserID:    userID,
		TextQuery: &question,
		Status:    models.StatusProcessing,
	}
	if err := h.repo.CreateRequest(r.Context(), dbReq); err != nil {
		slog.Error("Failed to create RAG request record", "error", err)
		// Non-fatal: continue serving the query even if tracking fails.
	}

	start := time.Now()

	embedding, err := h.embedQuestion(r.Context(), req.Question)
	if err != nil {
		slog.WarnContext(r.Context(), "Failed to embed RAG question for cache lookup", "request_id", requestID, "error", err)
	}

	// OR-logic hybrid cache lookup: accept if EITHER trigram OR vector similarity
	// exceeds its threshold. False positives (antonym pairs, different-diagnosis
	// same-domain) are filtered by the contradiction guard and LLM judge below.
	const trigramThreshold = 0.8
	const vectorThreshold = 0.88
	cached, err := h.repo.FindCachedAnswer(r.Context(), req.Question, embedding, trigramThreshold, vectorThreshold)
	if err != nil {
		slog.WarnContext(r.Context(), "Failed to lookup RAG cache", "request_id", requestID, "error", err)
	}
	if cached != nil {
		normalized := repository.NormalizeQuestion(req.Question)
		if repository.HasContradiction(normalized, cached.QuestionNormalized) {
			slog.Info("KB cache contradiction veto",
				"request_id", requestID,
				"incoming", normalized,
				"cached_question", cached.QuestionNormalized,
				"vector_similarity", cached.VectorSimilarity)
			cached = nil
		} else {
			equivalent, err := h.judgeEquivalence(r.Context(), normalized, cached.QuestionNormalized)
			if err != nil {
				slog.WarnContext(r.Context(), "LLM judge error — failing open",
					"request_id", requestID, "error", err)
			} else if !equivalent {
				slog.Info("KB cache LLM judge veto",
					"request_id", requestID,
					"incoming", normalized,
					"cached_question", cached.QuestionNormalized,
					"vector_similarity", cached.VectorSimilarity)
				cached = nil
			}
		}
	}
	if cached != nil {
		slog.Info("KB cache hit",
			"request_id", requestID,
			"similarity", cached.Similarity,
			"match_method", cached.MatchMethod,
			"cached_question", cached.QuestionNormalized)
		cacheBody := []byte(cached.Answer)
		elapsed := time.Since(start)
		h.saveRAGResponse(r, requestID, cacheBody, int(elapsed.Milliseconds()), "HIT", cached)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cacheBody)
		return
	}

	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.ragURL+"/query", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create upstream request")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(upstream)
	if err != nil {
		slog.Error("RAG service request failed", "error", err)
		h.markRequestFailed(r, requestID)
		writeError(w, http.StatusBadGateway, "RAG service unavailable")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			slog.WarnContext(r.Context(), "Failed to read RAG error response", "status", resp.StatusCode, "error", err)
		}
		if len(respBody) > 0 {
			slog.WarnContext(r.Context(), "RAG service returned error", "status", resp.StatusCode, "body", string(respBody))
		}
		h.markRequestFailed(r, requestID)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("RAG service error: %d", resp.StatusCode))
		return
	}

	// Read the full response so we can both persist it and return to client.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		slog.Warn("Failed to read RAG response", "error", err)
		h.markRequestFailed(r, requestID)
		writeError(w, http.StatusBadGateway, "failed to read RAG response")
		return
	}

	elapsed := time.Since(start)

	// Persist response record for performance tracking.
	h.saveRAGResponse(r, requestID, respBody, int(elapsed.Milliseconds()), "MISS", nil)

	// Store in hybrid cache for future similar questions.
	if err := h.repo.SaveCacheEntry(r.Context(), req.Question, embedding, string(respBody), ""); err != nil {
		slog.Warn("Failed to save KB cache entry", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (h *RAGHandler) markRequestFailed(r *http.Request, requestID uuid.UUID) {
	if err := h.repo.UpdateRequestStatus(r.Context(), requestID, models.StatusFailed); err != nil {
		slog.Error("Failed to mark RAG request as failed", "request_id", requestID, "error", err)
	}
}

const ragModel = "rag_query"

func (h *RAGHandler) saveRAGResponse(
	r *http.Request,
	requestID uuid.UUID,
	body []byte,
	elapsedMs int,
	cacheStatus string,
	cacheEntry *models.KBCacheEntry,
) {
	response := &models.Response{
		ID:               uuid.New(),
		RequestID:        requestID,
		Content:          string(body),
		Model:            ragModel,
		ProcessingTimeMs: elapsedMs,
		CacheStatus:      cacheStatus,
	}
	if cacheEntry != nil {
		response.CacheEntryID = &cacheEntry.ID
		response.CacheTrigramSimilarity = cacheEntry.TrigramSimilarity
		response.CacheVectorSimilarity = cacheEntry.VectorSimilarity
		response.CacheCombinedSimilarity = &cacheEntry.CombinedSimilarity
		response.CacheMatchMethod = cacheEntry.MatchMethod
	}
	if err := h.repo.CreateResponse(r.Context(), response); err != nil {
		slog.Error("Failed to save RAG response record", "request_id", requestID, "error", err)
		return
	}
	if err := h.repo.UpdateRequestStatus(r.Context(), requestID, models.StatusCompleted); err != nil {
		slog.Error("Failed to mark RAG request completed", "request_id", requestID, "error", err)
	}
}

const judgeModel = openai.GPT4oMini

// judgeEquivalence asks the LLM whether two questions require the same clinical answer.
// Returns true if equivalent (cache hit is safe), false if different topics.
// On any error the caller should fail-open and allow the cache hit.
func (h *RAGHandler) judgeEquivalence(ctx context.Context, incoming, cached string) (bool, error) {
	if h.judgeClient == nil {
		return true, nil // no key configured — skip judge, allow hit
	}

	prompt := fmt.Sprintf(
		"Question 1: %q\nQuestion 2: %q\n\n"+
			"Do both questions ask about the same clinical ECG topic and require the same answer?\n"+
			"If the questions ask about DIFFERENT NAMED diagnostic scoring systems or indices "+
			"(e.g. one asks about Cornell criteria and the other about Sokolow-Lyon index), answer NO.\n"+
			"Reply with exactly one word: YES or NO.",
		incoming, cached,
	)

	judgeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := h.judgeClient.CreateChatCompletion(judgeCtx, openai.ChatCompletionRequest{
		Model: judgeModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "You are a medical question classifier for an ECG knowledge base. " +
					"Your only task is to decide if two questions have the same clinical meaning and would need the same answer.",
			},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxTokens:   5,
		Temperature: 0,
	})
	if err != nil {
		return false, fmt.Errorf("judge call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return false, fmt.Errorf("judge returned no choices")
	}

	answer := strings.ToUpper(strings.TrimSpace(resp.Choices[0].Message.Content))
	return strings.HasPrefix(answer, "YES"), nil
}

func (h *RAGHandler) embedQuestion(ctx context.Context, question string) ([]float64, error) {
	body, err := json.Marshal(ragEmbedRequest{Text: question})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.ragURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embed endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embed endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed ragEmbedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("embed endpoint returned empty embedding")
	}
	return parsed.Embedding, nil
}

type ragFeedbackRequest struct {
	Question string `json:"question" validate:"required"`
	Answer   string `json:"answer"   validate:"required"`
	Rating   int    `json:"rating"   validate:"required,oneof=-1 1"`
}

// Feedback handles POST /v1/rag/feedback — stores user feedback on RAG answers.
func (h *RAGHandler) Feedback(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req ragFeedbackRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	feedback := &models.RAGFeedback{
		ID:       uuid.New(),
		UserID:   userID,
		Question: req.Question,
		Answer:   req.Answer,
		Rating:   req.Rating,
	}

	if err := h.repo.CreateRAGFeedback(r.Context(), feedback); err != nil {
		slog.Error("Failed to save RAG feedback", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save feedback")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}
