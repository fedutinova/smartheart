//go:build !opencv
// +build !opencv

package workers

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestEKGJobPayload_MarshalUnmarshal(t *testing.T) {
	original := EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis",
		UserID:       uuid.New().String(),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Unmarshal from JSON
	var unmarshaled EKGJobPayload
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Compare fields
	if original.ImageTempURL != unmarshaled.ImageTempURL {
		t.Errorf("Expected ImageTempURL %s, got %s", original.ImageTempURL, unmarshaled.ImageTempURL)
	}

	if original.Notes != unmarshaled.Notes {
		t.Errorf("Expected Notes %s, got %s", original.Notes, unmarshaled.Notes)
	}

	if original.UserID != unmarshaled.UserID {
		t.Errorf("Expected UserID %s, got %s", original.UserID, unmarshaled.UserID)
	}
}

func TestIsValidImageContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"image/jpeg", true},
		{"image/jpg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/bmp", true},
		{"image/tiff", true},
		{"application/pdf", true},
		{"text/plain", false},
		{"application/json", false},
		{"", false},
	}

	for _, test := range tests {
		result := isValidImageContentType(test.contentType)
		if result != test.expected {
			t.Errorf("Expected %v for content type %s, got %v", test.expected, test.contentType, result)
		}
	}
}

func TestCreateMockEKGJob(t *testing.T) {
	userID := uuid.New().String()
	imageURL := "http://example.com/test.jpg"

	payload := EKGJobPayload{
		ImageTempURL: imageURL,
		Notes:        "Test EKG analysis",
		UserID:       userID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal test payload: %v", err)
	}

	// Create a simple job structure for testing
	type TestJob struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Payload []byte `json:"payload"`
	}

	job := TestJob{
		ID:      uuid.New().String(),
		Type:    "ekg_analyze",
		Payload: payloadBytes,
	}

	if job.Type != "ekg_analyze" {
		t.Errorf("Expected job type 'ekg_analyze', got %s", job.Type)
	}

	var unmarshaledPayload EKGJobPayload
	err = json.Unmarshal(job.Payload, &unmarshaledPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal job payload: %v", err)
	}

	if unmarshaledPayload.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, unmarshaledPayload.UserID)
	}
}

// Benchmark tests
func BenchmarkEKGJobPayload_Marshal(b *testing.B) {
	payload := EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis with some notes",
		UserID:       uuid.New().String(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(payload)
	}
}

func BenchmarkEKGJobPayload_Unmarshal(b *testing.B) {
	payload := EKGJobPayload{
		ImageTempURL: "http://example.com/test.jpg",
		Notes:        "Test EKG analysis with some notes",
		UserID:       uuid.New().String(),
	}

	jsonData, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var unmarshaled EKGJobPayload
		json.Unmarshal(jsonData, &unmarshaled)
	}
}

func BenchmarkIsValidImageContentType(b *testing.B) {
	contentTypes := []string{
		"image/jpeg", "image/png", "image/gif", "image/webp",
		"image/bmp", "image/tiff", "application/pdf", "text/plain",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contentType := contentTypes[i%len(contentTypes)]
		isValidImageContentType(contentType)
	}
}
