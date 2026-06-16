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

// ragGenerationTimeout bounds a single RAG generation (upstream LLM call). It
// also caps the detached generation context so a client disconnect can't leave
// background work running indefinitely.
const ragGenerationTimeout = 120 * time.Second

// Hybrid KB cache match thresholds. A candidate is accepted if EITHER the
// trigram OR the vector cosine similarity clears its threshold.
const (
	kbTrigramThreshold = 0.8
	kbVectorThreshold  = 0.88
)

// Re-ask detection: if the same user asks a semantically similar question within
// reaskWindow, treat it as dissatisfaction with the previous answer and bypass
// the cache. Thresholds are slightly looser than the cache thresholds so a
// rephrasing ("the same thing in other words") still counts as a re-ask.
const (
	reaskWindow           = time.Hour
	reaskTrigramThreshold = 0.6
	reaskVectorThreshold  = 0.85
)

// NewRAGHandler creates a handler that forwards requests to the RAG service.
func NewRAGHandler(ragURL string, repo repository.Store, apiKey string) *RAGHandler {
	h := &RAGHandler{
		ragURL: ragURL,
		client: &http.Client{Timeout: ragGenerationTimeout},
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

	// Re-ask detection: if this user already asked a similar question within the
	// last hour, they are rephrasing because the previous answer didn't satisfy
	// them — bypass the cache and generate a fresh answer.
	reask, err := h.repo.HasRecentSimilarQuery(r.Context(), userID, req.Question, embedding, reaskWindow, reaskTrigramThreshold, reaskVectorThreshold)
	if err != nil {
		slog.WarnContext(r.Context(), "Failed to check recent re-ask; treating as not a re-ask",
			"request_id", requestID, "error", err)
	}
	// Record this query (best-effort) so subsequent re-asks are detected.
	if err := h.repo.LogRecentQuery(r.Context(), userID, req.Question, embedding); err != nil {
		slog.WarnContext(r.Context(), "Failed to log recent RAG query", "request_id", requestID, "error", err)
	}

	// OR-logic hybrid cache lookup: accept if EITHER trigram OR vector similarity
	// exceeds its threshold. False positives (antonym pairs, different-diagnosis
	// same-domain) are filtered by the contradiction guard and LLM judge below.
	// Skipped entirely on a re-ask.
	var cached *models.KBCacheEntry
	if reask {
		slog.Info("KB cache bypass: recent re-ask", "request_id", requestID)
	} else {
		cached, err = h.repo.FindCachedAnswer(r.Context(), req.Question, embedding, kbTrigramThreshold, kbVectorThreshold)
		if err != nil {
			slog.WarnContext(r.Context(), "Failed to lookup RAG cache", "request_id", requestID, "error", err)
		}
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
		h.saveRAGResponse(r.Context(), requestID, cacheBody, int(elapsed.Milliseconds()), "HIT", cached)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cacheBody)
		return
	}

	// Detach generation from the inbound request: once we commit to a cache MISS
	// we want the (paid) LLM answer to finish and populate the cache even if the
	// client disconnects mid-flight. The timeout matches the HTTP client budget.
	genCtx, cancelGen := context.WithTimeout(context.WithoutCancel(r.Context()), ragGenerationTimeout)
	defer cancelGen()

	upstream, err := http.NewRequestWithContext(genCtx, http.MethodPost, h.ragURL+"/query", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create upstream request")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(upstream)
	if err != nil {
		slog.Error("RAG service request failed", "error", err)
		h.markRequestFailed(genCtx, requestID)
		writeError(w, http.StatusBadGateway, "RAG service unavailable")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			slog.WarnContext(genCtx, "Failed to read RAG error response", "status", resp.StatusCode, "error", err)
		}
		if len(respBody) > 0 {
			slog.WarnContext(genCtx, "RAG service returned error", "status", resp.StatusCode, "body", string(respBody))
		}
		h.markRequestFailed(genCtx, requestID)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("RAG service error: %d", resp.StatusCode))
		return
	}

	// Read the full response so we can both persist it and return to client.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		slog.Warn("Failed to read RAG response", "error", err)
		h.markRequestFailed(genCtx, requestID)
		writeError(w, http.StatusBadGateway, "failed to read RAG response")
		return
	}

	elapsed := time.Since(start)

	// Persist response record for performance tracking. Uses a detached context
	// internally so the answer is recorded even if the client already left.
	h.saveRAGResponse(genCtx, requestID, respBody, int(elapsed.Milliseconds()), "MISS", nil)

	// Store in hybrid cache for future similar questions.
	cacheCtx, cancelCache := detachedWrite(genCtx)
	defer cancelCache()
	if err := h.repo.SaveCacheEntry(cacheCtx, req.Question, embedding, string(respBody), ""); err != nil {
		slog.Warn("Failed to save KB cache entry", "error", err)
	}

	// The client may have disconnected during generation; the write below is a
	// best-effort delivery to a still-connected client and fails harmlessly
	// otherwise. The answer is already persisted and cached above.

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

// detachedWrite returns a context for bookkeeping DB writes that must succeed
// even when the inbound request was canceled (client disconnect, browser
// navigation, axios timeout). Reusing the request context for these writes
// would fail with "context canceled" and leave request rows stuck in
// "processing". Mirrors the cleanup pattern in workers/ecg_handler.go.
func detachedWrite(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

func (h *RAGHandler) markRequestFailed(ctx context.Context, requestID uuid.UUID) {
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	if err := h.repo.UpdateRequestStatus(writeCtx, requestID, models.StatusFailed); err != nil {
		slog.Error("Failed to mark RAG request as failed", "request_id", requestID, "error", err)
	}
}

const ragModel = "rag_query"

func (h *RAGHandler) saveRAGResponse(
	ctx context.Context,
	requestID uuid.UUID,
	body []byte,
	elapsedMs int,
	cacheStatus string,
	cacheEntry *models.KBCacheEntry,
) {
	// On a cache MISS we already paid the LLM to generate this answer; persist it
	// with a detached context so a late client disconnect doesn't lose the
	// response and its cache entry.
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()

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
	if err := h.repo.CreateResponse(writeCtx, response); err != nil {
		slog.Error("Failed to save RAG response record", "request_id", requestID, "error", err)
		return
	}
	if err := h.repo.UpdateRequestStatus(writeCtx, requestID, models.StatusCompleted); err != nil {
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
		"Q1: %q\nQ2: %q\nSame clinical ECG topic requiring the same answer? "+
			"If different named scores/indices (e.g. Cornell vs Sokolow-Lyon), answer NO. "+
			"Reply exactly YES or NO.",
		incoming, cached,
	)

	judgeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := h.judgeClient.CreateChatCompletion(judgeCtx, openai.ChatCompletionRequest{
		Model: judgeModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "ECG KB question equivalence classifier. Output only YES or NO.",
			},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		MaxCompletionTokens: 5,
		Temperature:         0,
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

	// A dislike means the cached answer for this question is bad — invalidate it
	// so it stops being served and gets regenerated on the next query.
	if req.Rating == -1 {
		h.invalidateDislikedAnswer(r.Context(), req.Question)
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// invalidateDislikedAnswer expires the cache entry serving the disliked
// question. Best-effort and detached from the request: feedback should still
// succeed even if cache invalidation fails or the client disconnects.
func (h *RAGHandler) invalidateDislikedAnswer(ctx context.Context, question string) {
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()

	// Embedding is best-effort; ExpireCachedAnswer falls back to trigram-only
	// matching when it is empty.
	embedding, err := h.embedQuestion(writeCtx, question)
	if err != nil {
		slog.WarnContext(writeCtx, "Failed to embed disliked question; using trigram-only invalidation", "error", err)
	}
	if err := h.repo.ExpireCachedAnswer(writeCtx, question, embedding, kbTrigramThreshold, kbVectorThreshold); err != nil {
		slog.WarnContext(writeCtx, "Failed to invalidate disliked cache answer", "error", err)
	}
}
