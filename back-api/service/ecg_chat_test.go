package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

// expectTxRunsInline wires repo.RunTx to immediately invoke its callback
// with a nil tx, and repo.WithTx(tx) to return the same mock store so that
// CreateECGChatMessage expectations on the outer mock fire as expected.
func expectTxRunsInline(repo *repomocks.MockStore) {
	repo.EXPECT().
		RunTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(pgx.Tx) error) error {
			return fn(nil)
		})
	repo.EXPECT().WithTx(mock.Anything).Return(repository.Store(repo))
}

// ragHandler is a tiny test double for the RAG /query endpoint.
type ragHandler struct {
	statusCode int
	response   ragResponse
	lastBody   []byte
}

func (h *ragHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.lastBody, _ = io.ReadAll(r.Body)
	if h.statusCode == 0 {
		h.statusCode = http.StatusOK
	}
	w.WriteHeader(h.statusCode)
	_ = json.NewEncoder(w).Encode(h.response)
}

func newECGChatService(t *testing.T, h *ragHandler) (*ecgChatService, *repomocks.MockStore, *httptest.Server) {
	t.Helper()
	repo := repomocks.NewMockStore(t)
	mux := http.NewServeMux()
	mux.Handle("/query", h)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	svc := NewECGChatService(repo, server.URL).(*ecgChatService)
	return svc, repo, server
}

func newOwnedRequest(userID uuid.UUID) *models.Request {
	return &models.Request{
		ID:     uuid.New(),
		UserID: userID,
		Status: models.StatusCompleted,
	}
}

// --- SendMessage ---

func TestSendMessage_HappyPath(t *testing.T) {
	rag := &ragHandler{
		response: ragResponse{
			Answer: "Индекс Соколова-Лайона — это сумма S в V1 и R в V5/V6.",
			Sources: []ragSource{
				{DocName: "ESC Guidelines", ChunkIndex: 3, Score: 0.87, Preview: "ГЛЖ при сумме > 35 мм"},
			},
		},
	}
	svc, repo, _ := newECGChatService(t, rag)

	userID := uuid.New()
	req := newOwnedRequest(userID)

	repo.EXPECT().GetRequestByID(mock.Anything, req.ID).Return(req, nil)
	expectTxRunsInline(repo)
	repo.EXPECT().
		CreateECGChatMessage(mock.Anything, mock.MatchedBy(func(m *models.ECGChatMessage) bool {
			return m.Role == models.ECGChatRoleUser && m.Content == "Что такое индекс Соколова?"
		})).
		Return(nil)
	repo.EXPECT().
		CreateECGChatMessage(mock.Anything, mock.MatchedBy(func(m *models.ECGChatMessage) bool {
			return m.Role == models.ECGChatRoleAssistant &&
				strings.Contains(m.Content, "Соколова-Лайона") &&
				len(m.Citations) == 1
		})).
		Return(nil)

	reply, err := svc.SendMessage(context.Background(), req.ID, userID, "Что такое индекс Соколова?")
	require.NoError(t, err)
	assert.Equal(t, models.ECGChatRoleAssistant, reply.Role)
	assert.Contains(t, reply.Content, "Соколова-Лайона")
	require.Len(t, reply.Citations, 1)
	assert.Equal(t, "ESC Guidelines", reply.Citations[0].Title)

	// RAG должен получить контекст вместе с вопросом.
	var ragReq ragQuery
	require.NoError(t, json.Unmarshal(rag.lastBody, &ragReq))
	assert.Contains(t, ragReq.Question, "Контекст: пользователь обсуждает результаты своей ЭКГ")
	assert.Contains(t, ragReq.Question, "Что такое индекс Соколова?")
}

func TestSendMessage_EmptyContent_Validation(t *testing.T) {
	svc, _, _ := newECGChatService(t, &ragHandler{})

	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), "   ")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestSendMessage_TooLong_Validation(t *testing.T) {
	svc, _, _ := newECGChatService(t, &ragHandler{})

	long := strings.Repeat("a", ecgChatMaxQuestionLen+1)
	_, err := svc.SendMessage(context.Background(), uuid.New(), uuid.New(), long)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestSendMessage_ForeignRequest_NotFound(t *testing.T) {
	svc, repo, _ := newECGChatService(t, &ragHandler{})

	requester := uuid.New()
	owner := uuid.New()
	req := newOwnedRequest(owner)

	repo.EXPECT().GetRequestByID(mock.Anything, req.ID).Return(req, nil)

	_, err := svc.SendMessage(context.Background(), req.ID, requester, "Привет")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrRequestNotFound))
}

func TestSendMessage_RAGFails_NothingPersisted(t *testing.T) {
	rag := &ragHandler{statusCode: http.StatusInternalServerError}
	svc, repo, _ := newECGChatService(t, rag)

	userID := uuid.New()
	req := newOwnedRequest(userID)

	repo.EXPECT().GetRequestByID(mock.Anything, req.ID).Return(req, nil)
	// RAG fails before any persistence happens — neither user nor
	// assistant message should be written. mockery will fail the test
	// if CreateECGChatMessage or RunTx is called unexpectedly.

	_, err := svc.SendMessage(context.Background(), req.ID, userID, "Опасно ли это?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rag")
}

// --- GetMessages ---

func TestGetMessages_HappyPath(t *testing.T) {
	svc, repo, _ := newECGChatService(t, &ragHandler{})

	userID := uuid.New()
	req := newOwnedRequest(userID)
	expected := []models.ECGChatMessage{
		{ID: uuid.New(), RequestID: req.ID, UserID: userID, Role: models.ECGChatRoleUser, Content: "вопрос"},
		{ID: uuid.New(), RequestID: req.ID, UserID: userID, Role: models.ECGChatRoleAssistant, Content: "ответ"},
	}

	repo.EXPECT().GetRequestByID(mock.Anything, req.ID).Return(req, nil)
	repo.EXPECT().GetECGChatMessages(mock.Anything, req.ID, userID).Return(expected, nil)

	got, err := svc.GetMessages(context.Background(), req.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestGetMessages_ForeignRequest_NotFound(t *testing.T) {
	svc, repo, _ := newECGChatService(t, &ragHandler{})

	requester := uuid.New()
	owner := uuid.New()
	req := newOwnedRequest(owner)

	repo.EXPECT().GetRequestByID(mock.Anything, req.ID).Return(req, nil)

	_, err := svc.GetMessages(context.Background(), req.ID, requester)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrRequestNotFound))
}

// --- buildECGContextBlock ---

func TestBuildECGContextBlock_WithPatientAndSummary(t *testing.T) {
	age := 58
	sex := "male"
	req := &models.Request{
		ECGAge: &age,
		ECGSex: &sex,
		Response: &models.Response{
			Content: `{"analysis_type":"ekg_structured_v1","structured_result":{"interpretation":{"text_summary":"Синусовый ритм. Признаки ГЛЖ."}}}`,
		},
	}

	got := buildECGContextBlock(req)
	assert.Contains(t, got, "возраст 58 лет")
	assert.Contains(t, got, "пол мужской")
	assert.Contains(t, got, "Синусовый ритм. Признаки ГЛЖ.")
}

func TestBuildECGContextBlock_NoData(t *testing.T) {
	got := buildECGContextBlock(&models.Request{})
	assert.Equal(t, "Контекст: пользователь обсуждает результаты своей ЭКГ.", got)
}

// --- extractECGSummary ---

func TestExtractECGSummary_PrefersTextSummary(t *testing.T) {
	content := `{"analysis_type":"ekg_structured_v1","structured_result":{"interpretation":{"text_summary":"Норма"}}}`
	assert.Equal(t, "Норма", extractECGSummary(content))
}

func TestExtractECGSummary_FallsBackToHRAndAbnormal(t *testing.T) {
	content := `{
		"analysis_type":"ekg_structured_v1",
		"structured_result":{
			"rhythm":{"HR_bpm":73,"QRS_ms":110},
			"interpretation":{"summary":[{"label":"ГЛЖ","value":"Признаки","status":"positive"}]}
		}
	}`
	got := extractECGSummary(content)
	assert.Contains(t, got, "ЧСС 73 уд/мин")
	assert.Contains(t, got, "QRS 110 мс")
	assert.Contains(t, got, "ГЛЖ: Признаки")
}

func TestExtractECGSummary_InvalidJSON(t *testing.T) {
	assert.Equal(t, "", extractECGSummary("not json"))
	assert.Equal(t, "", extractECGSummary(`{"analysis_type":"other"}`))
}
