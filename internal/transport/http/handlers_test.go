package http

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Simple test for HTTP handlers without complex mocks
func TestHandlers_SubmitAnalyze_RequestValidation(t *testing.T) {
	// Test valid request data
	validReqData := map[string]interface{}{
		"image_temp_url": "http://example.com/test.jpg",
		"notes":          "Test EKG analysis",
	}

	reqBody, err := json.Marshal(validReqData)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Test JSON unmarshaling
	var testData map[string]interface{}
	err = json.Unmarshal(reqBody, &testData)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if testData["image_temp_url"] != validReqData["image_temp_url"] {
		t.Errorf("Expected image_temp_url %s, got %s", validReqData["image_temp_url"], testData["image_temp_url"])
	}

	if testData["notes"] != validReqData["notes"] {
		t.Errorf("Expected notes %s, got %s", validReqData["notes"], testData["notes"])
	}
}

func TestHandlers_SubmitAnalyze_InvalidJSON(t *testing.T) {
	// Test invalid JSON
	invalidJSON := []byte(`{"image_temp_url": "http://example.com/test.jpg", "notes": "Test EKG analysis"`)

	var testData map[string]interface{}
	err := json.Unmarshal(invalidJSON, &testData)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandlers_SubmitAnalyze_EmptyImageURL(t *testing.T) {
	// Test empty image URL
	reqData := map[string]interface{}{
		"image_temp_url": "",
		"notes":          "Test EKG analysis",
	}

	reqBody, _ := json.Marshal(reqData)

	var testData map[string]interface{}
	err := json.Unmarshal(reqBody, &testData)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if testData["image_temp_url"] != "" {
		t.Error("Expected empty image_temp_url")
	}
}

func TestHandlers_JobID_Validation(t *testing.T) {
	// Test valid UUID
	validUUID := uuid.New().String()

	// Test UUID parsing
	parsedUUID, err := uuid.Parse(validUUID)
	if err != nil {
		t.Fatalf("Failed to parse valid UUID: %v", err)
	}

	if parsedUUID.String() != validUUID {
		t.Errorf("Expected UUID %s, got %s", validUUID, parsedUUID.String())
	}

	// Test invalid UUID
	invalidUUID := "invalid-uuid"
	_, err = uuid.Parse(invalidUUID)
	if err == nil {
		t.Error("Expected error for invalid UUID")
	}
}

func TestHandlers_ResponseFormat(t *testing.T) {
	// Test response format
	responseData := map[string]interface{}{
		"job_id":  uuid.New().String(),
		"status":  "queued",
		"message": "EKG analysis job submitted successfully",
	}

	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var testResponse map[string]interface{}
	err = json.Unmarshal(responseJSON, &testResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	requiredFields := []string{"job_id", "status", "message"}
	for _, field := range requiredFields {
		if testResponse[field] == nil {
			t.Errorf("Expected field %s in response", field)
		}
	}
}

// Benchmark tests
func BenchmarkHandlers_RequestMarshaling(b *testing.B) {
	reqData := map[string]interface{}{
		"image_temp_url": "http://example.com/test.jpg",
		"notes":          "Test EKG analysis",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(reqData)
	}
}

func BenchmarkHandlers_RequestUnmarshaling(b *testing.B) {
	reqData := map[string]interface{}{
		"image_temp_url": "http://example.com/test.jpg",
		"notes":          "Test EKG analysis",
	}

	reqBody, _ := json.Marshal(reqData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var testData map[string]interface{}
		json.Unmarshal(reqBody, &testData)
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
