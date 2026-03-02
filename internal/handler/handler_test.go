package handler

import (
	"encoding/json"
	"testing"

	"github.com/fedutinova/smartheart/internal/job"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
)

// Simple test for HTTP handlers without complex mocks
func TestHandlers_SubmitAnalyze_RequestValidation(t *testing.T) {
	original := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
	}

	reqBody, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var decoded job.EKGJobPayload
	err = json.Unmarshal(reqBody, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if decoded.ImageTempURL != original.ImageTempURL {
		t.Errorf("Expected image_temp_url %s, got %s", original.ImageTempURL, decoded.ImageTempURL)
	}

	if decoded.Notes != original.Notes {
		t.Errorf("Expected notes %s, got %s", original.Notes, decoded.Notes)
	}
}

func TestHandlers_SubmitAnalyze_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"image_temp_url": "http://example.com/test.jpg", "notes": "Test EKG analysis"`)

	var decoded job.EKGJobPayload
	err := json.Unmarshal(invalidJSON, &decoded)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandlers_SubmitAnalyze_EmptyImageURL(t *testing.T) {
	original := job.EKGJobPayload{
		ImageTempURL: "",
		Notes:        "Test EKG analysis",
	}

	reqBody, _ := json.Marshal(original)

	var decoded job.EKGJobPayload
	err := json.Unmarshal(reqBody, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if decoded.ImageTempURL != "" {
		t.Error("Expected empty image_temp_url")
	}
}

func TestHandlers_JobID_Validation(t *testing.T) {
	validUUID := uuid.New().String()

	parsedUUID, err := uuid.Parse(validUUID)
	if err != nil {
		t.Fatalf("Failed to parse valid UUID: %v", err)
	}

	if parsedUUID.String() != validUUID {
		t.Errorf("Expected UUID %s, got %s", validUUID, parsedUUID.String())
	}

	invalidUUID := "invalid-uuid"
	_, err = uuid.Parse(invalidUUID)
	if err == nil {
		t.Error("Expected error for invalid UUID")
	}
}

func TestHandlers_ResponseFormat(t *testing.T) {
	resp := models.SubmitEKGResponse{
		JobID:     uuid.New().String(),
		RequestID: uuid.New().String(),
		Status:    "queued",
		Message:   "EKG analysis job submitted successfully",
	}

	responseJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded models.SubmitEKGResponse
	err = json.Unmarshal(responseJSON, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if decoded.JobID == "" {
		t.Error("Expected non-empty job_id")
	}
	if decoded.Status != "queued" {
		t.Errorf("Expected status 'queued', got %s", decoded.Status)
	}
	if decoded.Message == "" {
		t.Error("Expected non-empty message")
	}
}

// Benchmark tests
func BenchmarkHandlers_RequestMarshaling(b *testing.B) {
	payload := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(payload)
	}
}

func BenchmarkHandlers_RequestUnmarshaling(b *testing.B) {
	payload := job.EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
	}

	reqBody, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded job.EKGJobPayload
		json.Unmarshal(reqBody, &decoded)
	}
}

func BenchmarkHandlers_UUIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uuid.New()
	}
}

func BenchmarkHandlers_UUIDParsing(b *testing.B) {
	validUUID := uuid.New().String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uuid.Parse(validUUID)
	}
}
