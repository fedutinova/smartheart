package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// RAGHandler proxies knowledge-base queries to the RAG microservice.
type RAGHandler struct {
	ragURL string
	client *http.Client
}

// NewRAGHandler creates a handler that forwards requests to the RAG service.
func NewRAGHandler(ragURL string) *RAGHandler {
	return &RAGHandler{
		ragURL: ragURL,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

type ragQueryRequest struct {
	Question string `json:"question" validate:"required,min=2,max=2000"`
	NResults int    `json:"n_results,omitempty" validate:"omitempty,gte=1,lte=20"`
}

// Query handles POST /v1/rag/query — validates input, proxies to RAG service.
func (h *RAGHandler) Query(w http.ResponseWriter, r *http.Request) {
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

	body, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal request")
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
		writeError(w, http.StatusBadGateway, "RAG service unavailable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		slog.Warn("RAG service returned error", "status", resp.StatusCode, "body", string(respBody))
		writeError(w, http.StatusBadGateway, fmt.Sprintf("RAG service error: %d", resp.StatusCode))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Warn("failed to write RAG response", "error", err)
	}
}
