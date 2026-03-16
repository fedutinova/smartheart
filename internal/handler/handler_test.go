package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/fedutinova/smartheart/internal/config"
	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/repository"
	"github.com/fedutinova/smartheart/internal/storage"
	"github.com/fedutinova/smartheart/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- Mock implementations ---

// Compile-time interface checks.
var (
	_ repository.Store    = (*mockStore)(nil)
	_ job.Queue           = (*mockQueue)(nil)
	_ auth.SessionService = (*mockAuth)(nil)
	_ storage.Storage     = (*mockStorage)(nil)
)

type mockStore struct {
	testutil.MockRequestRepo // embeds RequestRepo methods

	createUserFn         func(ctx context.Context, user *models.User) error
	getUserByEmailFn     func(ctx context.Context, email string) (*models.User, error)
	getUserByIDFn        func(ctx context.Context, userID uuid.UUID) (*models.User, error)
	assignRoleFn         func(ctx context.Context, userID uuid.UUID, roleName string) error
	createRefreshTokenFn func(ctx context.Context, token *models.RefreshToken) error
	getRefreshTokenFn    func(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	revokeRefreshTokenFn func(ctx context.Context, tokenHash string) error
	loadRolePermsFn      func(ctx context.Context) (map[string][]string, error)
}

func (m *mockStore) CreateUser(ctx context.Context, user *models.User) error {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, user)
	}
	return nil
}
func (m *mockStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return nil, errors.New("not found")
}
func (m *mockStore) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, userID)
	}
	return nil, errors.New("not found")
}
func (m *mockStore) AssignRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error {
	if m.assignRoleFn != nil {
		return m.assignRoleFn(ctx, userID, roleName)
	}
	return nil
}
func (m *mockStore) CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error {
	if m.createRefreshTokenFn != nil {
		return m.createRefreshTokenFn(ctx, token)
	}
	return nil
}
func (m *mockStore) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	if m.getRefreshTokenFn != nil {
		return m.getRefreshTokenFn(ctx, tokenHash)
	}
	return nil, errors.New("not found")
}
func (m *mockStore) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	if m.revokeRefreshTokenFn != nil {
		return m.revokeRefreshTokenFn(ctx, tokenHash)
	}
	return nil
}
func (m *mockStore) LoadRolePermissions(ctx context.Context) (map[string][]string, error) {
	if m.loadRolePermsFn != nil {
		return m.loadRolePermsFn(ctx)
	}
	return nil, nil
}
func (m *mockStore) WithTx(_ pgx.Tx) repository.Store { return m }
func (m *mockStore) DB() *database.DB                 { return nil }

type mockQueue struct {
	enqueueFn func(ctx context.Context, j *job.Job) (uuid.UUID, error)
	statusFn  func(ctx context.Context, id uuid.UUID) (*job.Job, bool)
	lenVal    int
}

func (m *mockQueue) Enqueue(ctx context.Context, j *job.Job) (uuid.UUID, error) {
	if m.enqueueFn != nil {
		return m.enqueueFn(ctx, j)
	}
	id := uuid.New()
	j.ID = id
	j.Status = job.StatusQueued
	return id, nil
}
func (m *mockQueue) Status(ctx context.Context, id uuid.UUID) (*job.Job, bool) {
	if m.statusFn != nil {
		return m.statusFn(ctx, id)
	}
	return nil, false
}
func (m *mockQueue) StartConsumers(ctx context.Context, n int, handler job.Handler) {}
func (m *mockQueue) Len() int                                                       { return m.lenVal }
func (m *mockQueue) Close() error                                                   { return nil }

type mockAuth struct {
	pingErr error
}

func (m *mockAuth) Ping(ctx context.Context) error { return m.pingErr }
func (m *mockAuth) IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	return false, nil
}
func (m *mockAuth) GetLoginAttempts(ctx context.Context, email string) (int64, error) {
	return 0, nil
}
func (m *mockAuth) IncrLoginAttempts(ctx context.Context, email string, window time.Duration) (int64, error) {
	return 1, nil
}
func (m *mockAuth) ResetLoginAttempts(ctx context.Context, email string) error { return nil }
func (m *mockAuth) StoreRefreshToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	return nil
}
func (m *mockAuth) GetRefreshTokenUserID(ctx context.Context, tokenHash string) (string, error) {
	return "", errors.New("not found")
}
func (m *mockAuth) RevokeRefreshToken(ctx context.Context, tokenHash string) error { return nil }
func (m *mockAuth) StoreBlacklistedToken(ctx context.Context, tokenHash string, ttl time.Duration) error {
	return nil
}

type mockStorage struct{}

func (m *mockStorage) UploadFile(ctx context.Context, filename string, content io.Reader, contentType string) (*storage.UploadResult, error) {
	return &storage.UploadResult{Key: "test-key", URL: "http://test/test-key"}, nil
}
func (m *mockStorage) GetPresignedURL(ctx context.Context, key string, exp time.Duration) (string, error) {
	return "http://test/" + key, nil
}
func (m *mockStorage) DeleteFile(ctx context.Context, key string) error { return nil }
func (m *mockStorage) GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("data")), "application/octet-stream", nil
}

// --- Helpers ---

type testOpts struct {
	queue    job.Queue
	repo     repository.Store
	sessions auth.SessionService
	storage  storage.Storage
	config   config.Config
}

func newTestHandler(opts ...func(*testOpts)) *Handler {
	o := &testOpts{
		queue:    &mockQueue{},
		repo:     &mockStore{},
		sessions: &mockAuth{},
		storage:  &mockStorage{},
		config:   config.Config{JWT: config.JWTConfig{Secret: "test-secret", Issuer: "test"}},
	}
	for _, fn := range opts {
		fn(o)
	}
	return NewHandler(o.queue, o.repo, o.sessions, o.storage, o.config)
}

func withAuthContext(r *http.Request, userID uuid.UUID, roles []string) *http.Request {
	claims := &auth.Claims{UserID: userID.String(), Roles: roles}
	return r.WithContext(auth.NewContext(r.Context(), claims))
}

func addChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- Health tests ---

func TestHealth_ReturnsOK(t *testing.T) {
	h := newTestHandler()
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
	h := newTestHandler()
	userID := uuid.New()

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
	h := newTestHandler()
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
	h := newTestHandler()
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
	h := newTestHandler()
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
	h := newTestHandler()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "http://example.com/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	// No auth context means empty userID string → uuid.Parse("") fails → 400
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitEKGAnalyze_RepoError(t *testing.T) {
	h := newTestHandler(func(o *testOpts) {
		o.repo = &mockStore{
			MockRequestRepo: testutil.MockRequestRepo{
				CreateRequestFn: func(ctx context.Context, req *models.Request) error {
					return errors.New("db down")
				},
			},
		}
	})
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

func TestSubmitEKGAnalyze_QueueError(t *testing.T) {
	h := newTestHandler(func(o *testOpts) {
		o.queue = &mockQueue{
			enqueueFn: func(ctx context.Context, j *job.Job) (uuid.UUID, error) {
				return uuid.Nil, errors.New("queue full")
			},
		}
	})
	userID := uuid.New()

	body, _ := json.Marshal(map[string]string{"image_temp_url": "http://example.com/ekg.jpg"})
	req := httptest.NewRequest("POST", "/v1/ekg/analyze", bytes.NewReader(body))
	req = withAuthContext(req, userID, []string{"user"})
	w := httptest.NewRecorder()

	h.EKG.SubmitEKGAnalyze(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// --- GetJob tests ---

func TestGetJob_NotFound(t *testing.T) {
	h := newTestHandler()
	userID := uuid.New()
	jobID := uuid.New()

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
	h := newTestHandler()
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
	userID := uuid.New()
	jobID := uuid.New()

	payloadBytes, _ := json.Marshal(map[string]string{"user_id": userID.String()})
	testJob := &job.Job{
		ID:      jobID,
		Type:    job.TypeEKGAnalyze,
		Status:  job.StatusRunning,
		Payload: payloadBytes,
	}

	h := newTestHandler(func(o *testOpts) {
		o.queue = &mockQueue{
			statusFn: func(ctx context.Context, id uuid.UUID) (*job.Job, bool) {
				if id == jobID {
					return testJob, true
				}
				return nil, false
			},
		}
	})

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
