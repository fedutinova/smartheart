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

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

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
)

// --- Helpers ---

type testDeps struct {
	authSvc       *svcmocks.MockAuthService
	passwordSvc   *svcmocks.MockPasswordService
	submissionSvc *svcmocks.MockSubmissionService
	requestSvc    *svcmocks.MockRequestService
	paymentSvc    *svcmocks.MockPaymentService
	queue         *jobmocks.MockQueue
	repo          *repomocks.MockStore
	sessions      *authmocks.MockSessionService
	storage       *storagemocks.MockStorage
	config        config.Config
}

func newTestDeps(t testing.TB) *testDeps {
	return &testDeps{
		authSvc:       svcmocks.NewMockAuthService(t),
		passwordSvc:   svcmocks.NewMockPasswordService(t),
		submissionSvc: svcmocks.NewMockSubmissionService(t),
		requestSvc:    svcmocks.NewMockRequestService(t),
		paymentSvc:    svcmocks.NewMockPaymentService(t),
		queue:         jobmocks.NewMockQueue(t),
		repo:          repomocks.NewMockStore(t),
		sessions:      authmocks.NewMockSessionService(t),
		storage:       storagemocks.NewMockStorage(t),
		config:        config.Config{JWT: config.JWTConfig{Secret: "test-secret", Issuer: "test"}},
	}
}

func (d *testDeps) handler() *Handler {
	return NewHandler(d.authSvc, d.passwordSvc, d.submissionSvc, d.requestSvc, d.paymentSvc, d.queue, d.repo, d.sessions, d.storage, notify.NewHub(), d.config, Middlewares{})
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
	req := httptest.NewRequest("GET", "/health", http.NoBody)
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

func TestSubmitECGAnalyze_Success(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.submissionSvc.EXPECT().
		SubmitECG(mock.Anything, mock.Anything, "https://8.8.8.8/ekg.jpg", mock.Anything).
		Return(&service.SubmittedJob{JobID: uuid.New(), RequestID: uuid.New(), Status: "queued"}, nil)

	h := d.handler()

	body, _ := json.Marshal(map[string]string{
		"image_temp_url": "https://8.8.8.8/ekg.jpg",
	})
	req := httptest.NewRequest("POST", "/v1/ecg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SubmitECGResponse
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

func TestSubmitECGAnalyze_EmptyBody(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/v1/ecg/analyze", strings.NewReader(""))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitECGAnalyze_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	req := httptest.NewRequest("POST", "/v1/ecg/analyze", strings.NewReader("{invalid"))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitECGAnalyze_MissingImageURL(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()
	userID := uuid.New()

	// Missing image_temp_url is caught by struct tag validation (required)
	body, _ := json.Marshal(make(map[string]string))
	req := httptest.NewRequest("POST", "/v1/ecg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitECGAnalyze_NoAuthContext(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "https://8.8.8.8/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ecg/analyze", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitECGAnalyze_ServiceError(t *testing.T) {
	d := newTestDeps(t)

	d.submissionSvc.EXPECT().
		SubmitECG(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, apperr.ErrInternal)

	h := d.handler()
	userID := uuid.New()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "https://8.8.8.8/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ecg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitECGAnalyze(w, req)

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

	req := httptest.NewRequest("GET", "/v1/jobs/"+jobID.String(), http.NoBody)
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

	req := httptest.NewRequest("GET", "/v1/jobs/not-a-uuid", http.NoBody)
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
		Type:   job.TypeECGAnalyze,
		Status: job.StatusRunning,
	}

	d.requestSvc.EXPECT().
		GetJobStatus(mock.Anything, jobID, mock.Anything).
		Return(testJob, nil)

	h := d.handler()

	req := httptest.NewRequest("GET", "/v1/jobs/"+jobID.String(), http.NoBody)
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
	original := job.ECGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New(),
		RequestID:    uuid.New(),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded job.ECGJobPayload
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

func TestSubmitECGResponse_Roundtrip(t *testing.T) {
	resp := SubmitECGResponse{
		JobID:     uuid.New(),
		RequestID: uuid.New(),
		Status:    "queued",
		Message:   "EKG analysis job submitted successfully",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SubmitECGResponse
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
	h := d.handler()

	// Missing username and password caught by struct tag validation
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
	h := d.handler()

	// Invalid email caught by struct tag validation (email)
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
	h := d.handler()

	// Short password caught by struct tag validation (min=10)
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
	h := d.handler()
	longPassword := strings.Repeat("a", 73)

	// Password too long caught by struct tag validation (max=72)
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

func TestRegister_Conflict(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	d.authSvc.EXPECT().
		Register(mock.Anything, "alice", "alice@example.com", "securepassword123").
		Return(uuid.Nil, apperr.ErrConflict)

	body, _ := json.Marshal(map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "securepassword123",
	})
	req := httptest.NewRequest("POST", "/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Auth.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
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
	h := d.handler()

	// Missing password caught by struct tag validation (required)
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

	// Access token returned in JSON body
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["access_token"] == "" {
		t.Error("expected non-empty access_token in body")
	}
	if _, hasRefresh := resp["refresh_token"]; hasRefresh {
		t.Error("refresh_token must NOT be in JSON body — it should be in a cookie")
	}

	// Refresh token set as httpOnly cookie
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if refreshCookie.Value != "refresh-token" {
		t.Errorf("expected cookie value 'refresh-token', got %q", refreshCookie.Value)
	}
	if !refreshCookie.HttpOnly {
		t.Error("refresh_token cookie must be HttpOnly")
	}
}

func TestRefresh_MissingCookie(t *testing.T) {
	d := newTestDeps(t)
	h := d.handler()

	// No refresh_token cookie → 401
	req := httptest.NewRequest("POST", "/v1/auth/refresh", http.NoBody)
	w := httptest.NewRecorder()

	h.Auth.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	d := newTestDeps(t)

	d.authSvc.EXPECT().
		Refresh(mock.Anything, "invalid-token").
		Return(nil, apperr.ErrInvalidToken)

	h := d.handler()

	req := httptest.NewRequest("POST", "/v1/auth/refresh", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "invalid-token"})
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

	req := httptest.NewRequest("POST", "/v1/auth/logout", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "some-refresh-token"})
	req = withAuthContext(req, userID, []string{"user"})
	accessToken, _ := auth.NewToken("test-secret", "test", userID.String(), []string{"user"}, 15*time.Minute)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()

	h.Auth.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Cookie must be cleared
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie to be cleared")
	}
	if refreshCookie.MaxAge >= 0 {
		t.Errorf("expected MaxAge < 0 (cookie deletion), got %d", refreshCookie.MaxAge)
	}
}

func TestLogout_NoCookie(t *testing.T) {
	d := newTestDeps(t)
	userID := uuid.New()

	d.authSvc.EXPECT().
		Logout(mock.Anything, "", mock.Anything, mock.Anything).
		Return(nil)

	h := d.handler()

	req := httptest.NewRequest("POST", "/v1/auth/logout", http.NoBody)
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.Auth.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Benchmarks ---

func BenchmarkHandlers_RequestMarshaling(b *testing.B) {
	payload := job.ECGJobPayload{
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
	payload := job.ECGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New(),
		RequestID:    uuid.New(),
	}

	reqBody, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded job.ECGJobPayload
		_ = json.Unmarshal(reqBody, &decoded)
	}
}
