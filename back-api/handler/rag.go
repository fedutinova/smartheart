package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// RAGHandler proxies knowledge-base queries to the RAG microservice.
type RAGHandler struct {
	ragURL    string
	client    *http.Client
	repo      repository.Store
	mockDelay time.Duration // >0 enables mock mode (no RAG service call)
}

// NewRAGHandler creates a handler that forwards requests to the RAG service.
// If RAG_MOCK_DELAY env is set (e.g. "3s"), RAG calls are simulated locally.
func NewRAGHandler(ragURL string, repo repository.Store) *RAGHandler {
	h := &RAGHandler{
		ragURL: ragURL,
		client: &http.Client{Timeout: 120 * time.Second},
		repo:   repo,
	}
	if v := os.Getenv("RAG_MOCK_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			h.mockDelay = d
			slog.Warn("RAG mock mode enabled", "delay", d)
		}
	}
	return h
}

type ragQueryRequest struct {
	Question string `json:"question"            validate:"required,min=2,max=2000"`
	NResults int    `json:"n_results,omitempty" validate:"omitempty,gte=1,lte=20"`
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

	// Semantic cache lookup.
	const cacheThreshold = 0.8
	if cached, err := h.repo.FindCachedAnswer(r.Context(), req.Question, cacheThreshold); err == nil && cached != nil {
		slog.Info("KB cache hit",
			"request_id", requestID,
			"similarity", cached.Similarity,
			"cached_question", cached.QuestionNormalized)
		cacheBody := []byte(cached.Answer)
		elapsed := time.Since(start)
		h.saveRAGResponse(r, requestID, cacheBody, int(elapsed.Milliseconds()))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cacheBody)
		return
	}

	// Mock mode: simulate RAG response with fixed delay.
	if h.mockDelay > 0 {
		select {
		case <-time.After(h.mockDelay):
		case <-r.Context().Done():
			h.markRequestFailed(r, requestID)
			writeError(w, http.StatusGatewayTimeout, "request cancelled")
			return
		}
		mockBody := []byte(`{"answer":"Мок-ответ для нагрузочного тестирования.","sources":[],"elapsed_ms":` +
			fmt.Sprintf("%.0f", float64(h.mockDelay.Milliseconds())) + `}`)
		elapsed := time.Since(start)
		h.saveRAGResponse(r, requestID, mockBody, int(elapsed.Milliseconds()))
		if err := h.repo.SaveCacheEntry(r.Context(), req.Question, string(mockBody), ""); err != nil {
			slog.Warn("Failed to save KB cache entry (mock)", "error", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(mockBody)
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
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		slog.Warn("RAG service returned error", "status", resp.StatusCode, "body", string(respBody))
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
	h.saveRAGResponse(r, requestID, respBody, int(elapsed.Milliseconds()))

	// Store in semantic cache for future similar questions.
	if err := h.repo.SaveCacheEntry(r.Context(), req.Question, string(respBody), ""); err != nil {
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

func (h *RAGHandler) saveRAGResponse(r *http.Request, requestID uuid.UUID, body []byte, elapsedMs int) {
	response := &models.Response{
		ID:               uuid.New(),
		RequestID:        requestID,
		Content:          string(body),
		Model:            ragModel,
		ProcessingTimeMs: elapsedMs,
	}
	if err := h.repo.CreateResponse(r.Context(), response); err != nil {
		slog.Error("Failed to save RAG response record", "request_id", requestID, "error", err)
		return
	}
	if err := h.repo.UpdateRequestStatus(r.Context(), requestID, models.StatusCompleted); err != nil {
		slog.Error("Failed to mark RAG request completed", "request_id", requestID, "error", err)
	}
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
