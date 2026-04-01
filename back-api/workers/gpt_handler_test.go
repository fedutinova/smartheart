package workers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/fedutinova/smartheart/back-api/gpt"
	"github.com/fedutinova/smartheart/back-api/job"
	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

// --- HandleGPTJob tests ---

func TestHandleGPTJob_WrongJobType(t *testing.T) {
	repo := repomocks.NewMockRequestRepo(t)
	h := &GPTWorker{repo: repo}

	j := &job.Job{
		ID:   uuid.New(),
		Type: job.TypeECGAnalyze,
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
	repo := repomocks.NewMockRequestRepo(t)
	h := &GPTWorker{repo: repo}

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
	repo := repomocks.NewMockRequestRepo(t)
	repo.EXPECT().
		UpdateRequestStatus(mock.Anything, mock.Anything, models.StatusProcessing).
		Return(errors.New("db down"))

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

// --- formatECGFallback tests ---

func TestFormatEKGFallback_WithNotesAndQuery(t *testing.T) {
	ekg := &models.ECGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "patient notes",
	}

	result := formatECGFallback(ekg, "query text")

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
	ekg := &models.ECGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "same text",
	}

	result := formatECGFallback(ekg, "same text")

	// When notes == textQuery, the query should NOT be duplicated
	count := strings.Count(result, "same text")
	if count != 1 {
		t.Errorf("expected notes to appear once (no duplication), appeared %d times", count)
	}
}

func TestFormatEKGFallback_EmptyNotes(t *testing.T) {
	ekg := &models.ECGResponseContent{
		Timestamp: "2026-01-01T00:00:00Z",
		Notes:     "",
	}

	result := formatECGFallback(ekg, "")

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
	requestID := uuid.New()
	userID := uuid.New()
	textQuery := "test query"

	repo := repomocks.NewMockRequestRepo(t)
	repo.EXPECT().
		GetRequestByID(mock.Anything, requestID).
		Return(&models.Request{
			ID:        requestID,
			UserID:    userID,
			TextQuery: &textQuery,
		}, nil)
	repo.EXPECT().
		GetRecentRequestsWithResponses(mock.Anything, userID, mock.Anything).
		Return(nil, nil)

	h := &GPTWorker{repo: repo}
	payload := gpt.JobPayload{
		RequestID: requestID,
		TextQuery: textQuery,
		UserID:    userID,
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

	ecgContent := &models.ECGResponseContent{
		AnalysisType: models.ECGModelDirect,
		Notes:        "patient notes",
		Timestamp:    "2026-01-01T00:00:00Z",
		GPTRequestID: requestID.String(),
	}
	ekgJSON, _ := ecgContent.Marshal()

	repo := repomocks.NewMockRequestRepo(t)
	repo.EXPECT().
		GetRequestByID(mock.Anything, requestID).
		Return(&models.Request{ID: requestID, UserID: userID}, nil)
	repo.EXPECT().
		GetRecentRequestsWithResponses(mock.Anything, userID, mock.Anything).
		Return([]models.Request{
			{ID: requestID},
			{
				ID:     ekgRequestID,
				UserID: userID,
				Response: &models.Response{
					Content: ekgJSON,
				},
			},
		}, nil)

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
	requestID := uuid.New()

	repo := repomocks.NewMockRequestRepo(t)
	repo.EXPECT().
		GetRequestByID(mock.Anything, requestID).
		Return(nil, errors.New("not found"))

	h := &GPTWorker{repo: repo}
	payload := gpt.JobPayload{
		RequestID: requestID,
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
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := isPrivateIP(ip); got != tt.want {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
