package workers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/fedutinova/smartheart/internal/gpt"
	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/fedutinova/smartheart/internal/testutil"
	"github.com/google/uuid"
)

// mockRequestRepo is an alias for the shared test mock.
type mockRequestRepo = testutil.MockRequestRepo

// --- HandleGPTJob tests ---

func TestHandleGPTJob_WrongJobType(t *testing.T) {
	h := &GPTWorker{repo: &mockRequestRepo{}}

	j := &job.Job{
		ID:   uuid.New(),
		Type: job.TypeEKGAnalyze,
	}

	err := h.HandleGPTJob(context.Background(), j)
	if err == nil {
		t.Fatal("expected error for wrong job type")
	}
	if got := err.Error(); got != "unexpected job type: ekg_analyze" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestHandleGPTJob_InvalidPayload(t *testing.T) {
	h := &GPTWorker{repo: &mockRequestRepo{}}

	j := &job.Job{
		ID:      uuid.New(),
		Type:    job.TypeGPTProcess,
		Payload: []byte("{invalid json"),
	}

	err := h.HandleGPTJob(context.Background(), j)
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestHandleGPTJob_UpdateStatusFails(t *testing.T) {
	repo := &mockRequestRepo{
		UpdateRequestStatusFn: func(_ context.Context, _ uuid.UUID, status string) error {
			if status == models.StatusProcessing {
				return errors.New("db down")
			}
			return nil
		},
	}
	h := &GPTWorker{repo: repo}

	payload := gpt.JobPayload{
		RequestID: uuid.New(),
		TextQuery: "test query",
		UserID:    uuid.New(),
	}
	payloadBytes, _ := json.Marshal(payload)

	j := &job.Job{
		ID:      uuid.New(),
		Type:    job.TypeGPTProcess,
		Payload: payloadBytes,
	}

	err := h.HandleGPTJob(context.Background(), j)
	if err == nil {
		t.Fatal("expected error when status update fails")
	}
	if got := err.Error(); got != "failed to update request status: db down" {
		t.Errorf("unexpected error: %s", got)
	}
}

// --- buildEKGPrompt tests ---

func TestBuildEKGPrompt_WithNotes(t *testing.T) {
	result := buildEKGPrompt("user notes")

	if result == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.Contains(result, "user notes") {
		t.Error("expected prompt to contain user notes")
	}
	if !strings.Contains(result, "Additional context from user") {
		t.Error("expected prompt to contain context header")
	}
	if !strings.Contains(result, "educational and technical analysis") {
		t.Error("expected prompt to contain disclaimer")
	}
}

func TestBuildEKGPrompt_WithoutNotes(t *testing.T) {
	result := buildEKGPrompt("")

	if result == "" {
		t.Fatal("expected non-empty prompt")
	}
	if strings.Contains(result, "Additional context from user") {
		t.Error("should not contain context header when no notes")
	}
	if !strings.Contains(result, "educational and technical analysis") {
		t.Error("expected prompt to contain disclaimer")
	}
}

// --- formatEKGFallback tests ---

func TestFormatEKGFallback_WithNotesAndQuery(t *testing.T) {
	ekg := &models.EKGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "patient notes",
	}

	result := formatEKGFallback(ekg, "query text")

	if !strings.Contains(result, "Автоматический анализ ЭКГ") {
		t.Error("expected Russian header")
	}
	if !strings.Contains(result, "2026-01-01T00:00:00Z") {
		t.Error("expected timestamp")
	}
	if !strings.Contains(result, "patient notes") {
		t.Error("expected notes")
	}
	if !strings.Contains(result, "query text") {
		t.Error("expected query text")
	}
}

func TestFormatEKGFallback_SameNotesAndQuery(t *testing.T) {
	ekg := &models.EKGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "same text",
	}

	result := formatEKGFallback(ekg, "same text")

	// When notes == textQuery, the query should NOT be duplicated
	count := strings.Count(result, "same text")
	if count != 1 {
		t.Errorf("expected notes to appear once (no duplication), appeared %d times", count)
	}
}

func TestFormatEKGFallback_EmptyNotes(t *testing.T) {
	ekg := &models.EKGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "",
	}

	result := formatEKGFallback(ekg, "")

	if !strings.Contains(result, "2026-01-01T00:00:00Z") {
		t.Error("expected timestamp")
	}
	if strings.Contains(result, "Примечания пользователя") {
		t.Error("should not contain notes section when notes are empty")
	}
}

// --- formatBasicFallback tests ---

func TestFormatBasicFallback_WithQuery(t *testing.T) {
	result := formatBasicFallback("my query")

	if !strings.Contains(result, "my query") {
		t.Error("expected query in fallback")
	}
	if !strings.Contains(result, "Автоматический анализ") {
		t.Error("expected Russian header")
	}
}

func TestFormatBasicFallback_EmptyQuery(t *testing.T) {
	result := formatBasicFallback("")

	if strings.Contains(result, "Контекст запроса") {
		t.Error("should not contain query section when empty")
	}
	if !strings.Contains(result, "попробуйте повторить") {
		t.Error("expected retry message")
	}
}

// --- createFallbackResponse tests ---

func TestCreateFallbackResponse_NoEKGData(t *testing.T) {
	repo := &mockRequestRepo{
		GetRequestByIDFn: func(_ context.Context, _ uuid.UUID) (*models.Request, error) {
			textQuery := "test query"
			return &models.Request{
				ID:        uuid.New(),
				UserID:    uuid.New(),
				TextQuery: &textQuery,
			}, nil
		},
		GetRecentRequestsWithResponsesFn: func(_ context.Context, _ uuid.UUID, _ int) ([]models.Request, error) {
			return nil, nil // no related requests
		},
	}

	h := &GPTWorker{repo: repo}
	payload := gpt.JobPayload{
		RequestID: uuid.New(),
		TextQuery: "test query",
		UserID:    uuid.New(),
	}

	result, err := h.createFallbackResponse(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty fallback")
	}
	if !strings.Contains(result, "test query") {
		t.Error("expected query in basic fallback")
	}
}

func TestCreateFallbackResponse_WithEKGResponse(t *testing.T) {
	requestID := uuid.New()
	userID := uuid.New()
	ekgRequestID := uuid.New()

	ekgContent := &models.EKGResponseContent{
		AnalysisType: models.EKGModelDirect,
		Notes:        "patient notes",
		Timestamp:    "2026-01-01T00:00:00Z",
		GPTRequestID: requestID.String(),
	}
	ekgJSON, _ := ekgContent.Marshal()

	repo := &mockRequestRepo{
		GetRequestByIDFn: func(_ context.Context, id uuid.UUID) (*models.Request, error) {
			if id == requestID {
				return &models.Request{ID: requestID, UserID: userID}, nil
			}
			return nil, errors.New("not found")
		},
		GetRecentRequestsWithResponsesFn: func(_ context.Context, _ uuid.UUID, _ int) ([]models.Request, error) {
			return []models.Request{
				{ID: requestID},
				{
					ID:     ekgRequestID,
					UserID: userID,
					Response: &models.Response{
						Content: ekgJSON,
					},
				},
			}, nil
		},
	}

	h := &GPTWorker{repo: repo}
	payload := gpt.JobPayload{
		RequestID: requestID,
		TextQuery: "analyze this",
		UserID:    userID,
	}

	result, err := h.createFallbackResponse(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "patient notes") {
		t.Error("expected EKG notes in fallback")
	}
}

func TestCreateFallbackResponse_RequestNotFound(t *testing.T) {
	repo := &mockRequestRepo{
		GetRequestByIDFn: func(_ context.Context, _ uuid.UUID) (*models.Request, error) {
			return nil, errors.New("not found")
		},
	}

	h := &GPTWorker{repo: repo}
	payload := gpt.JobPayload{
		RequestID: uuid.New(),
		UserID:    uuid.New(),
	}

	_, err := h.createFallbackResponse(context.Background(), payload)
	if err == nil {
		t.Fatal("expected error when request not found")
	}
}

// --- validateImageURL tests ---

func TestValidateImageURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/image.jpg", false},
		{"valid http", "http://example.com/image.jpg", false},
		{"empty", "", true},
		{"ftp scheme", "ftp://example.com/file", true},
		{"localhost", "http://localhost/image.jpg", true},
		{"127.0.0.1", "http://127.0.0.1/image.jpg", true},
		{"ipv6 loopback", "http://[::1]/image.jpg", true},
		{"0.0.0.0", "http://0.0.0.0/image.jpg", true},
		{"file scheme", "file:///etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImageURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// --- isPrivateIP tests ---

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 192.168.x", "192.168.1.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"link-local", "169.254.1.1", true},
		{"unspecified v4", "0.0.0.0", true},
		{"public IP", "8.8.8.8", false},
		{"public IP 2", "1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := isPrivateIP(ip); got != tt.want {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// --- helpers ---

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
