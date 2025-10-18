//go:build !opencv
// +build !opencv

package ekg

import (
	"context"
	"image"
	"testing"
)

func TestNewEKGPreprocessor(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	if preprocessor == nil {
		t.Fatal("Expected preprocessor to be created")
	}

	if preprocessor.TargetWidth != 800 {
		t.Errorf("Expected TargetWidth to be 800, got %d", preprocessor.TargetWidth)
	}

	if preprocessor.TargetHeight != 600 {
		t.Errorf("Expected TargetHeight to be 600, got %d", preprocessor.TargetHeight)
	}
}

func TestExtractSignalFeatures_EmptyContour(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	// Test with empty contour
	features := preprocessor.ExtractSignalFeatures([]image.Point{})

	if features["error"] == nil {
		t.Error("Expected error for empty contour")
	}
}

func TestExtractSignalFeatures_InsufficientPoints(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	// Test with insufficient points
	insufficientContour := []image.Point{{10, 10}}
	features := preprocessor.ExtractSignalFeatures(insufficientContour)

	if features["error"] == nil {
		t.Error("Expected error for insufficient contour points")
	}
}

func TestExtractSignalFeatures_ValidContour(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	// Test with valid contour
	validContour := []image.Point{
		{10, 50}, {20, 45}, {30, 55}, {40, 50}, {50, 60},
		{60, 45}, {70, 55}, {80, 50}, {90, 65},
	}

	features := preprocessor.ExtractSignalFeatures(validContour)

	// Check required features
	requiredFields := []string{
		"points_count", "signal_width", "amplitude_range",
		"baseline", "standard_deviation", "bounding_box",
	}

	for _, field := range requiredFields {
		if features[field] == nil {
			t.Errorf("Expected feature %s to be present", field)
		}
	}

	// Check specific values
	if features["points_count"] != len(validContour) {
		t.Errorf("Expected points_count to be %d, got %v", len(validContour), features["points_count"])
	}

	if features["signal_width"] != 80 { // 90 - 10
		t.Errorf("Expected signal_width to be 80, got %v", features["signal_width"])
	}
}

func TestPreprocessImage_EmptyInput(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	// Test with empty image data
	result, err := preprocessor.PreprocessImage(context.Background(), []byte{})

	if err == nil {
		t.Error("Expected error for empty image data")
	}

	if result != nil {
		t.Error("Expected nil result for empty image data")
	}
}

func TestPreprocessImage_InvalidData(t *testing.T) {
	preprocessor := NewEKGPreprocessor()

	// Test with invalid image data
	result, err := preprocessor.PreprocessImage(context.Background(), []byte("invalid image data"))

	if err == nil {
		t.Error("Expected error for invalid image data")
	}

	if result != nil {
		t.Error("Expected nil result for invalid image data")
	}
}

// Benchmark tests
func BenchmarkExtractSignalFeatures(b *testing.B) {
	preprocessor := NewEKGPreprocessor()

	// Create a large contour for benchmarking
	contour := make([]image.Point, 1000)
	for i := 0; i < 1000; i++ {
		contour[i] = image.Point{X: i, Y: 50 + (i % 20)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessor.ExtractSignalFeatures(contour)
	}
}

func BenchmarkNewEKGPreprocessor(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewEKGPreprocessor()
	}
}
