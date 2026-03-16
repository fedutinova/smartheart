package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	authmocks "github.com/fedutinova/smartheart/back-api/auth/mocks"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/job"
	jobmocks "github.com/fedutinova/smartheart/back-api/job/mocks"
	"github.com/fedutinova/smartheart/back-api/notify"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
	"github.com/fedutinova/smartheart/back-api/service"
	svcmocks "github.com/fedutinova/smartheart/back-api/service/mocks"
	storagemocks "github.com/fedutinova/smartheart/back-api/storage/mocks"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// --- Helpers ---

type testDeps struct {
	authSvc       *svcmocks.MockAuthService
	submissionSvc *svcmocks.MockSubmissionService
	requestSvc    *svcmocks.MockRequestService
	queue         *jobmocks.MockQueue
	repo          *repomocks.MockStore
	sessions      *authmocks.MockSessionService
	storage       *storagemocks.MockStorage
	config        config.Config
}

func newTestDeps(t testing.TB) *testDeps {
	return &testDeps{
		authSvc:       svcmocks.NewMockAuthService(t),
		submissionSvc: svcmocks.NewMockSubmissionService(t),
		requestSvc:    svcmocks.NewMockRequestService(t),
		queue:         jobmocks.NewMockQueue(t),
		repo:          repomocks.NewMockStore(t),
		sessions:      authmocks.NewMockSessionService(t),
		storage:       storagemocks.NewMockStorage(t),
		config:        config.Config{JWT: config.JWTConfig{Secret: "test-secret", Issuer: "test"}},
	}
}

func (d *testDeps) handler() *Handler {
	return NewHandler(d.authSvc, d.submissionSvc, d.requestSvc, d.queue, d.repo, d.sessions, d.storage, notify.NewHub(), d.config)
}

func withAuthContext(r *http.Request, userID uuid.UUID, roles []string) *http.Request {
	claims := &auth.Claims{
		UserID: userID.String(),
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	return r.WithContext(auth.NewContext(r.Context(), claims))
}

func addChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- Health tests ---

func TestHealth_ReturnsOK(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.Healthz.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", status.Status)
	}
}

// --- EKG handler tests ---

func TestSubmitEKGAnalyze_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.submissionSvc.EXPECT().
		SubmitEKG(mock.Anything, mock.Anything, "http://example.com/ekg.jpg", "test notes").
		Return(&service.SubmittedJob{JobID: uuid.New(), RequestID: uuid.New(), Status: "queued"}, nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"image_temp_url": "http://example.com/ekg.jpg",
		"notes":          "test notes",
	})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SubmitEKGResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.JobID == uuid.Nil {
		t.Error("expected non-nil job_id")
	}
	if resp.RequestID == uuid.Nil {
		t.Error("expected non-nil request_id")
	}
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestSubmitEKGAnalyze_EmptyBody(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/v1/ekg/analyze", strings.NewReader(""))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitEKGAnalyze_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/v1/ekg/analyze", strings.NewReader("{invalid"))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitEKGAnalyze_MissingImageURL(t *testing.T) {
	d := newTestDeps(t)

	d.submissionSvc.EXPECT().
		SubmitEKG(mock.Anything, mock.Anything, "", mock.Anything).
		Return(nil, apperr.ErrValidation)

	h := d.handler()
	userID := uuid.New()

	body, _ := json.Marshal(map[string]string{"notes": "test"})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitEKGAnalyze_NoAuthContext(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "http://example.com/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitEKGAnalyze_ServiceError(t *testing.T) {
	d := newTestDeps(t)

	d.submissionSvc.EXPECT().
		SubmitEKG(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, apperr.ErrInternal)

	h := d.handler()
	userID := uuid.New()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "http://example.com/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- GetJob tests ---

func TestGetJob_NotFound(t *testing.T) {
	d := newTestDeps(t)
	jobID := uuid.New()

	d.requestSvc.EXPECT().
		GetJobStatus(mock.Anything, jobID, mock.Anything).
		Return(nil, apperr.ErrJobNotFound)

	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("GET", "/v1/jobs/"+jobID.String(), nil)
	req = withAuthContext(req, userID, []string{"user"})
	req = addChiURLParam(req, "id", jobID.String())
	w := httptest.NewRecorder()

	h.Request.GetJob(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetJob_BadID(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("GET", "/v1/jobs/not-a-uuid", nil)
	req = withAuthContext(req, userID, []string{"user"})
	req = addChiURLParam(req, "id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.Request.GetJob(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetJob_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()
	jobID := uuid.New()

	testJob := &job.Job{
		ID:     jobID,
		Type:   job.TypeEKGAnalyze,
		Status: job.StatusRunning,
	}

	d.requestSvc.EXPECT().
		GetJobStatus(mock.Anything, jobID, mock.Anything).
		Return(testJob, nil)

	h := d.handler()

	req := httptest.NewRequest("GET", "/v1/jobs/"+jobID.String(), nil)
	req = withAuthContext(req, userID, []string{"user"})
	req = addChiURLParam(req, "id", jobID.String())
	w := httptest.NewRecorder()

	h.Request.GetJob(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Serialization tests ---

func TestEKGPayload_Roundtrip(t *testing.T) {
	original := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New(),
		RequestID:    uuid.New(),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded job.EKGJobPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ImageTempURL != original.ImageTempURL {
		t.Errorf("ImageTempURL: got %s, want %s", decoded.ImageTempURL, original.ImageTempURL)
	}
	if decoded.UserID != original.UserID {
		t.Errorf("UserID: got %s, want %s", decoded.UserID, original.UserID)
	}
	if decoded.RequestID != original.RequestID {
		t.Errorf("RequestID: got %s, want %s", decoded.RequestID, original.RequestID)
	}
}

func TestSubmitEKGResponse_Roundtrip(t *testing.T) {
	resp := SubmitEKGResponse{
		JobID:     uuid.New(),
		RequestID: uuid.New(),
		Status:    "queued",
		Message:   "EKG analysis job submitted successfully",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SubmitEKGResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.JobID != resp.JobID {
		t.Errorf("JobID: got %s, want %s", decoded.JobID, resp.JobID)
	}
	if decoded.Status != resp.Status {
		t.Errorf("Status: got %s, want %s", decoded.Status, resp.Status)
	}
}

// --- Auth handler tests ---

func TestRegister_MissingFields(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Register(mock.Anything, mock.Anything, "alice@example.com", mock.Anything).
		Return(uuid.Nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Register(mock.Anything, "alice", "not-an-email", "securepassword123").
		Return(uuid.Nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"username": "alice",
		"email":    "not-an-email",
		"password": "securepassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Register(mock.Anything, "alice", "alice@example.com", "short").
		Return(uuid.Nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "short",
	})
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegister_PasswordTooLong(t *testing.T) {
	d := newTestDeps(t)
	longPassword := strings.Repeat("a", 73)

	d.authSvc.EXPECT().
		Register(mock.Anything, "alice", "alice@example.com", longPassword).
		Return(uuid.Nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": longPassword,
	})
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader("{bad"))
	w := httptest.NewRecorder()

	h.Auth.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Login(mock.Anything, "alice@example.com", "").
		Return(nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Login(mock.Anything, "noone@example.com", "securepassword123").
		Return(nil, apperr.ErrInvalidCredentials)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"email":    "noone@example.com",
		"password": "securepassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Login(mock.Anything, "alice@example.com", "wrongpassword").
		Return(nil, apperr.ErrInvalidCredentials)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"email":    "alice@example.com",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Login(mock.Anything, "alice@example.com", "securepassword123").
		Return(&auth.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}, nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"email":    "alice@example.com",
		"password": "securepassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var tokens auth.TokenPair
	if err := json.NewDecoder(w.Body).Decode(&tokens); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if tokens.RefreshToken == "" {
		t.Error("expected non-empty refresh_token")
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Refresh(mock.Anything, "").
		Return(nil, apperr.ErrValidation)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Refresh(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Refresh(mock.Anything, "invalid-token").
		Return(nil, apperr.ErrInvalidToken)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{"refresh_token": "invalid-token"})
	req := httptest.NewRequest("POST", "/v1/auth/refresh", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.authSvc.EXPECT().
		Logout(mock.Anything, "some-refresh-token", mock.Anything, mock.Anything).
		Return(nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{"refresh_token": "some-refresh-token"})
	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	accessToken, _ := auth.NewToken("test-secret", "test", userID.String(), []string{"user"}, 15*time.Minute)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	h.Auth.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogout_EmptyBody(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.authSvc.EXPECT().
		Logout(mock.Anything, "", mock.Anything, mock.Anything).
		Return(nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/v1/auth/logout", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.Auth.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Benchmarks ---

func BenchmarkHandlers_RequestMarshaling(b *testing.B) {
	payload := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New(),
		RequestID:    uuid.New(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(payload)
	}
}

func BenchmarkHandlers_RequestUnmarshaling(b *testing.B) {
	payload := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New(),
		RequestID:    uuid.New(),
	}

	reqBody, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded job.EKGJobPayload
		_ = json.Unmarshal(reqBody, &decoded)
	}
}
