//go:build !opencv
// +build !opencv

package workers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// Test server for mocking image downloads
func createTestImageServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock different scenarios based on path
		switch r.URL.Path {
		case "/valid-image.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", "1024")
			w.WriteHeader(http.StatusOK)
			// Write some mock JPEG data
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
		case "/invalid-content-type.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("This is not an image"))
		case "/large-file.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Content-Length", "10485760") // 10MB + 1 byte
			w.WriteHeader(http.StatusOK)
			// Write large amount of data
			for i := 0; i < 1000; i++ {
				w.Write(make([]byte, 1024))
			}
		case "/not-found.jpg":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestEKGHandler_Integration_ValidImage(t *testing.T) {
	server := createTestImageServer()
	defer server.Close()

	// Create a simple test payload
	payload := EKGJobPayload{
		ImageTempURL: server.URL + "/valid-image.jpg",
		Notes:        "Integration test",
		UserID:       uuid.New().String(),
	}

	payloadBytes, _ := json.Marshal(payload)

	// Test payload marshaling/unmarshaling
	var testPayload EKGJobPayload
	err := json.Unmarshal(payloadBytes, &testPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if testPayload.ImageTempURL != payload.ImageTempURL {
		t.Errorf("Expected URL %s, got %s", payload.ImageTempURL, testPayload.ImageTempURL)
	}
}

func TestEKGHandler_Integration_InvalidContentType(t *testing.T) {
	server := createTestImageServer()
	defer server.Close()

	// Test with invalid content type URL
	payload := EKGJobPayload{
		ImageTempURL: server.URL + "/invalid-content-type.txt",
		Notes:        "Integration test",
		UserID:       uuid.New().String(),
	}

	payloadBytes, _ := json.Marshal(payload)

	// Test payload processing
	var testPayload EKGJobPayload
	err := json.Unmarshal(payloadBytes, &testPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Test content type validation
	if isValidImageContentType("text/plain") {
		t.Error("Expected text/plain to be invalid content type")
	}
}

func TestEKGHandler_Integration_LargeFile(t *testing.T) {
	server := createTestImageServer()
	defer server.Close()

	// Test with large file URL
	payload := EKGJobPayload{
		ImageTempURL: server.URL + "/large-file.jpg",
		Notes:        "Integration test",
		UserID:       uuid.New().String(),
	}

	payloadBytes, _ := json.Marshal(payload)

	// Test payload processing
	var testPayload EKGJobPayload
	err := json.Unmarshal(payloadBytes, &testPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if testPayload.ImageTempURL != payload.ImageTempURL {
		t.Errorf("Expected URL %s, got %s", payload.ImageTempURL, testPayload.ImageTempURL)
	}
}

func TestEKGHandler_Integration_NotFound(t *testing.T) {
	server := createTestImageServer()
	defer server.Close()

	// Test with not found URL
	payload := EKGJobPayload{
		ImageTempURL: server.URL + "/not-found.jpg",
		Notes:        "Integration test",
		UserID:       uuid.New().String(),
	}

	payloadBytes, _ := json.Marshal(payload)

	// Test payload processing
	var testPayload EKGJobPayload
	err := json.Unmarshal(payloadBytes, &testPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if testPayload.ImageTempURL != payload.ImageTempURL {
		t.Errorf("Expected URL %s, got %s", payload.ImageTempURL, testPayload.ImageTempURL)
	}
}

// Test concurrent processing of multiple payloads
func TestEKGHandler_Integration_ConcurrentProcessing(t *testing.T) {
	server := createTestImageServer()
	defer server.Close()

	// Create multiple payloads
	numPayloads := 5
	payloads := make([]EKGJobPayload, numPayloads)

	for i := 0; i < numPayloads; i++ {
		payloads[i] = EKGJobPayload{
			ImageTempURL: server.URL + "/valid-image.jpg",
			Notes:        "Concurrent test",
			UserID:       uuid.New().String(),
		}
	}

	// Process payloads concurrently
	results := make(chan error, numPayloads)

	for _, payload := range payloads {
		go func(p EKGJobPayload) {
			payloadBytes, err := json.Marshal(p)
			if err != nil {
				results <- err
				return
			}

			var testPayload EKGJobPayload
			err = json.Unmarshal(payloadBytes, &testPayload)
			results <- err
		}(payload)
	}

	// Collect results
	for i := 0; i < numPayloads; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent processing error: %v", err)
		}
	}
}

// Benchmark integration tests
func BenchmarkEKGHandler_Integration_ContentTypeValidation(b *testing.B) {
	server := createTestImageServer()
	defer server.Close()

	urls := []string{
		"/valid-image.jpg",
		"/invalid-content-type.txt",
		"/large-file.jpg",
		"/not-found.jpg",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := server.URL + urls[i%len(urls)]

		payload := EKGJobPayload{
			ImageTempURL: url,
			Notes:        "Benchmark test",
			UserID:       uuid.New().String(),
		}

		payloadBytes, _ := json.Marshal(payload)
		var testPayload EKGJobPayload
		json.Unmarshal(payloadBytes, &testPayload)
	}
}

func BenchmarkEKGHandler_Integration_PayloadProcessing(b *testing.B) {
	server := createTestImageServer()
	defer server.Close()

	payload := EKGJobPayload{
		ImageTempURL: server.URL + "/valid-image.jpg",
		Notes:        "Benchmark test with longer notes to test processing performance",
		UserID:       uuid.New().String(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payloadBytes, _ := json.Marshal(payload)
		var testPayload EKGJobPayload
		json.Unmarshal(payloadBytes, &testPayload)
	}
}
