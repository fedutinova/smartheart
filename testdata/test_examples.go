package testdata

import (
	"encoding/json"
	"fmt"
)

// EKGTestExamples provides example test data for EKG analysis
type EKGTestExamples struct {
	ValidRequests   []EKGTestRequest `json:"valid_requests"`
	InvalidRequests []EKGTestRequest `json:"invalid_requests"`
	TestImages      []TestImageInfo  `json:"test_images"`
}

type EKGTestRequest struct {
	Name        string `json:"name"`
	ImageURL    string `json:"image_url"`
	Notes       string `json:"notes"`
	Expected    string `json:"expected"`
	Description string `json:"description"`
}

type TestImageInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	Format      string `json:"format"`
	Description string `json:"description"`
}

// GetEKGTestExamples returns example test data
func GetEKGTestExamples() *EKGTestExamples {
	return &EKGTestExamples{
		ValidRequests: []EKGTestRequest{
			{
				Name:        "basic_ekg_analysis",
				ImageURL:    "http://example.com/ekg1.jpg",
				Notes:       "Patient EKG from emergency room",
				Expected:    "success",
				Description: "Basic EKG analysis with valid image URL",
			},
			{
				Name:        "ekg_with_detailed_notes",
				ImageURL:    "http://example.com/ekg2.png",
				Notes:       "Patient: John Doe, Age: 45, Symptoms: Chest pain, Duration: 2 hours",
				Expected:    "success",
				Description: "EKG analysis with detailed patient notes",
			},
			{
				Name:        "multiple_lead_ekg",
				ImageURL:    "http://example.com/ekg12lead.jpg",
				Notes:       "12-lead EKG for comprehensive analysis",
				Expected:    "success",
				Description: "12-lead EKG analysis",
			},
		},
		InvalidRequests: []EKGTestRequest{
			{
				Name:        "empty_image_url",
				ImageURL:    "",
				Notes:       "Test with empty URL",
				Expected:    "error",
				Description: "Request with empty image URL should fail",
			},
			{
				Name:        "invalid_url_format",
				ImageURL:    "not-a-valid-url",
				Notes:       "Test with invalid URL format",
				Expected:    "error",
				Description: "Request with invalid URL format should fail",
			},
			{
				Name:        "non_image_content_type",
				ImageURL:    "http://example.com/document.pdf",
				Notes:       "Test with non-image content",
				Expected:    "error",
				Description: "Request with non-image content type should fail",
			},
			{
				Name:        "large_file",
				ImageURL:    "http://example.com/huge-image.jpg",
				Notes:       "Test with oversized image",
				Expected:    "error",
				Description: "Request with oversized image should fail",
			},
		},
		TestImages: []TestImageInfo{
			{
				Name:        "test_ekg",
				Type:        "ekg",
				Size:        1024,
				Format:      "JPEG",
				Description: "Synthetic EKG signal for testing",
			},
			{
				Name:        "test_image",
				Type:        "png",
				Size:        512,
				Format:      "PNG",
				Description: "Simple colored square for testing",
			},
			{
				Name:        "large_image",
				Type:        "large",
				Size:        10485760, // 10MB
				Format:      "JPEG",
				Description: "Large image for size limit testing",
			},
			{
				Name:        "corrupted_image",
				Type:        "corrupted",
				Size:        256,
				Format:      "JPEG (corrupted)",
				Description: "Corrupted image data for error testing",
			},
		},
	}
}

// GetTestRequestJSON returns a test request as JSON string
func GetTestRequestJSON(requestType string) (string, error) {
	examples := GetEKGTestExamples()

	var request EKGTestRequest
	found := false

	switch requestType {
	case "valid":
		if len(examples.ValidRequests) > 0 {
			request = examples.ValidRequests[0]
			found = true
		}
	case "invalid":
		if len(examples.InvalidRequests) > 0 {
			request = examples.InvalidRequests[0]
			found = true
		}
	default:
		return "", fmt.Errorf("unknown request type: %s", requestType)
	}

	if !found {
		return "", fmt.Errorf("no test request found for type: %s", requestType)
	}

	// Create API request format
	apiRequest := map[string]interface{}{
		"image_temp_url": request.ImageURL,
		"notes":          request.Notes,
	}

	jsonData, err := json.Marshal(apiRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	return string(jsonData), nil
}

// GetTestImageData returns test image data by type
func GetTestImageData(imageType string) []byte {
	switch imageType {
	case "ekg":
		return CreateTestEKGImage()
	case "png":
		return CreateTestPNGImage()
	case "large":
		return CreateLargeImage()
	case "corrupted":
		return CreateCorruptedImage()
	default:
		return CreateTestEKGImage()
	}
}

// PrintTestExamples prints all test examples to console
func PrintTestExamples() {
	examples := GetEKGTestExamples()

	fmt.Println("=== EKG Test Examples ===")

	fmt.Println("\nValid Requests:")
	for i, req := range examples.ValidRequests {
		fmt.Printf("  %d. %s: %s\n", i+1, req.Name, req.Description)
	}

	fmt.Println("\nInvalid Requests:")
	for i, req := range examples.InvalidRequests {
		fmt.Printf("  %d. %s: %s\n", i+1, req.Name, req.Description)
	}

	fmt.Println("\nTest Images:")
	for i, img := range examples.TestImages {
		fmt.Printf("  %d. %s (%s): %s\n", i+1, img.Name, img.Format, img.Description)
	}

	fmt.Println("\n=== Example API Request ===")
	validRequest, _ := GetTestRequestJSON("valid")
	fmt.Printf("Valid Request JSON:\n%s\n", validRequest)

	fmt.Println("\n=== Example API Response ===")
	response := map[string]interface{}{
		"job_id":  "123e4567-e89b-12d3-a456-426614174000",
		"status":  "queued",
		"message": "EKG analysis job submitted successfully",
	}

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("Response JSON:\n%s\n", string(responseJSON))
}

// GetTestConfig returns test configuration
func GetTestConfig() map[string]interface{} {
	return map[string]interface{}{
		"test_server": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
			"url":  "http://localhost:8080",
		},
		"test_data": map[string]interface{}{
			"image_dir": "./testdata/images",
			"max_size":  10485760, // 10MB
			"timeout":   30,       // seconds
		},
		"test_users": map[string]interface{}{
			"valid_user": map[string]interface{}{
				"email":    "test@example.com",
				"password": "testpassword",
				"role":     "user",
			},
			"admin_user": map[string]interface{}{
				"email":    "admin@example.com",
				"password": "adminpassword",
				"role":     "admin",
			},
		},
	}
}

// ValidateTestRequest validates a test request
func ValidateTestRequest(request EKGTestRequest) []string {
	var errors []string

	if request.ImageURL == "" {
		errors = append(errors, "image_url is required")
	}

	if len(request.Notes) > 4000 {
		errors = append(errors, "notes too long (max 4000 characters)")
	}

	if request.Expected == "" {
		errors = append(errors, "expected result is required")
	}

	return errors
}

// GetTestMetrics returns test performance metrics
func GetTestMetrics() map[string]interface{} {
	return map[string]interface{}{
		"performance": map[string]interface{}{
			"image_processing_time_ms": 2000,
			"http_request_time_ms":     100,
			"database_query_time_ms":   50,
			"total_pipeline_time_ms":   3000,
		},
		"limits": map[string]interface{}{
			"max_file_size_mb":    10,
			"max_notes_length":    4000,
			"max_concurrent_jobs": 4,
			"request_timeout_sec": 30,
		},
		"coverage": map[string]interface{}{
			"unit_tests":        "95%",
			"integration_tests": "85%",
			"api_tests":         "90%",
			"overall":           "88%",
		},
	}
}
