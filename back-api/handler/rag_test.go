package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

// ragTestServer returns an httptest server standing in for the RAG microservice,
// answering /embed and /query with minimal valid JSON.
func ragTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/embed", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float64{0.1, 0.2, 0.3}, "model": "test", "dimensions": 3,
		})
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answer": "fresh answer", "sources": []any{},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newRAGHandler(srv *httptest.Server, repo *repomocks.MockStore) *RAGHandler {
	return &RAGHandler{
		ragURL: srv.URL,
		client: &http.Client{Timeout: ragGenerationTimeout},
		repo:   repo,
	}
}

func ragQueryRequestBody(t *testing.T, question string) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(map[string]any{"question": question})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func TestRAGQuery_ReaskBypassesCache(t *testing.T) {
	srv := ragTestServer(t)
	repo := repomocks.NewMockStore(t)
	userID := uuid.New()

	repo.EXPECT().CreateRequest(mock.Anything, mock.Anything).Return(nil)
	// Re-ask detected → cache must be bypassed (FindCachedAnswer NOT expected).
	repo.EXPECT().HasRecentSimilarQuery(mock.Anything, userID, "что такое глж", mock.Anything,
		reaskWindow, reaskTrigramThreshold, reaskVectorThreshold).Return(true, nil)
	repo.EXPECT().LogRecentQuery(mock.Anything, userID, "что такое глж", mock.Anything).Return(nil)
	// Generation persists a MISS response + cache entry.
	repo.EXPECT().CreateResponse(mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().UpdateRequestStatus(mock.Anything, mock.Anything, models.StatusCompleted).Return(nil)
	repo.EXPECT().SaveCacheEntry(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	h := newRAGHandler(srv, repo)
	r := withAuthContext(httptest.NewRequest(http.MethodPost, "/v1/rag/query", ragQueryRequestBody(t, "что такое глж")), userID, nil)
	w := httptest.NewRecorder()

	h.Query(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Cache"); got != "MISS" {
		t.Errorf("expected X-Cache MISS on re-ask, got %q", got)
	}
	// repomocks.NewMockStore(t) auto-asserts that FindCachedAnswer was never called.
}

func TestRAGQuery_CacheHitWhenNotReask(t *testing.T) {
	srv := ragTestServer(t)
	repo := repomocks.NewMockStore(t)
	userID := uuid.New()

	repo.EXPECT().CreateRequest(mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().HasRecentSimilarQuery(mock.Anything, userID, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	repo.EXPECT().LogRecentQuery(mock.Anything, userID, mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().FindCachedAnswer(mock.Anything, mock.Anything, mock.Anything, kbTrigramThreshold, kbVectorThreshold).
		Return(&models.KBCacheEntry{
			ID:                 uuid.New(),
			QuestionNormalized: "что такое глж",
			Answer:             `{"answer":"cached","sources":[]}`,
		}, nil)
	repo.EXPECT().CreateResponse(mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().UpdateRequestStatus(mock.Anything, mock.Anything, models.StatusCompleted).Return(nil)

	h := newRAGHandler(srv, repo)
	r := withAuthContext(httptest.NewRequest(http.MethodPost, "/v1/rag/query", ragQueryRequestBody(t, "что такое глж")), userID, nil)
	w := httptest.NewRecorder()

	h.Query(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Cache"); got != "HIT" {
		t.Errorf("expected X-Cache HIT, got %q", got)
	}
}

func TestRAGFeedback_DislikeInvalidatesCache(t *testing.T) {
	srv := ragTestServer(t)
	repo := repomocks.NewMockStore(t)
	userID := uuid.New()

	repo.EXPECT().CreateRAGFeedback(mock.Anything, mock.Anything).Return(nil)
	repo.EXPECT().ExpireCachedAnswer(mock.Anything, "плохой вопрос", mock.Anything,
		kbTrigramThreshold, kbVectorThreshold).Return(nil)

	h := newRAGHandler(srv, repo)
	body, _ := json.Marshal(map[string]any{"question": "плохой вопрос", "answer": "плохой ответ", "rating": -1})
	r := withAuthContext(httptest.NewRequest(http.MethodPost, "/v1/rag/feedback", bytes.NewReader(body)), userID, nil)
	w := httptest.NewRecorder()

	h.Feedback(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRAGFeedback_LikeDoesNotInvalidateCache(t *testing.T) {
	srv := ragTestServer(t)
	repo := repomocks.NewMockStore(t)
	userID := uuid.New()

	repo.EXPECT().CreateRAGFeedback(mock.Anything, mock.Anything).Return(nil)
	// ExpireCachedAnswer must NOT be called for a positive rating; MockStore(t)
	// fails the test if it is.

	h := newRAGHandler(srv, repo)
	body, _ := json.Marshal(map[string]any{"question": "хороший вопрос", "answer": "хороший ответ", "rating": 1})
	r := withAuthContext(httptest.NewRequest(http.MethodPost, "/v1/rag/feedback", bytes.NewReader(body)), userID, nil)
	w := httptest.NewRecorder()

	h.Feedback(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}
}
