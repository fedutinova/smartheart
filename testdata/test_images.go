package testdata

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

// CreateTestEKGImage creates a simple test EKG-like image
func CreateTestEKGImage() []byte {
	// Create a simple EKG-like pattern
	width, height := 800, 600
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill background with white
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	// Draw EKG-like signal
	baseline := height / 2
	for x := 0; x < width; x++ {
		y := baseline + int(20*sin(float64(x)*0.1)) // Simple sine wave
		if y >= 0 && y < height {
			img.Set(x, y, color.RGBA{0, 0, 0, 255}) // Black signal
		}

		// Add some spikes (QRS complexes)
		if x%100 == 0 {
			for spike := 0; spike < 20; spike++ {
				spikeY := baseline - spike
				if spikeY >= 0 && spikeY < height {
					img.Set(x, spikeY, color.RGBA{0, 0, 0, 255})
				}
			}
		}
	}

	// Encode as JPEG
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// CreateTestPNGImage creates a simple test PNG image
func CreateTestPNGImage() []byte {
	// Create a simple colored square
	width, height := 100, 100
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with red
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	// Encode as PNG
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// CreateLargeImage creates a large test image (for size limit testing)
func CreateLargeImage() []byte {
	// Create a large image that exceeds size limits
	width, height := 2000, 2000
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with random-like pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create some pattern to make it larger when compressed
			r := uint8((x + y) % 256)
			g := uint8((x * y) % 256)
			b := uint8((x - y + 256) % 256)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	// Encode as JPEG with high quality to make it large
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	if err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// CreateCorruptedImage creates corrupted image data
func CreateCorruptedImage() []byte {
	// Create valid JPEG header but corrupt data
	validJPEG := CreateTestEKGImage()

	// Corrupt the middle part
	corrupted := make([]byte, len(validJPEG))
	copy(corrupted, validJPEG)

	// Replace middle section with random data
	for i := len(corrupted) / 4; i < len(corrupted)*3/4; i++ {
		corrupted[i] = 0xFF // Corrupt byte
	}

	return corrupted
}

// SaveTestImagesToFile saves test images to files for manual testing
func SaveTestImagesToFile(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Save EKG test image
	ekgData := CreateTestEKGImage()
	if err := os.WriteFile(filepath.Join(dir, "test_ekg.jpg"), ekgData, 0644); err != nil {
		return err
	}

	// Save PNG test image
	pngData := CreateTestPNGImage()
	if err := os.WriteFile(filepath.Join(dir, "test_image.png"), pngData, 0644); err != nil {
		return err
	}

	// Save large test image
	largeData := CreateLargeImage()
	if err := os.WriteFile(filepath.Join(dir, "large_image.jpg"), largeData, 0644); err != nil {
		return err
	}

	// Save corrupted test image
	corruptedData := CreateCorruptedImage()
	if err := os.WriteFile(filepath.Join(dir, "corrupted_image.jpg"), corruptedData, 0644); err != nil {
		return err
	}

	return nil
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

// GetTestImageInfo returns information about test images
func GetTestImageInfo() map[string]interface{} {
	return map[string]interface{}{
		"ekg": map[string]interface{}{
			"size":    len(CreateTestEKGImage()),
			"format":  "JPEG",
			"purpose": "EKG signal analysis testing",
		},
		"png": map[string]interface{}{
			"size":    len(CreateTestPNGImage()),
			"format":  "PNG",
			"purpose": "General image testing",
		},
		"large": map[string]interface{}{
			"size":    len(CreateLargeImage()),
			"format":  "JPEG",
			"purpose": "Size limit testing",
		},
		"corrupted": map[string]interface{}{
			"size":    len(CreateCorruptedImage()),
			"format":  "JPEG (corrupted)",
			"purpose": "Error handling testing",
		},
	}
}

// Simple sine function for creating EKG-like waves
func sin(x float64) float64 {
	// Simple sine approximation
	if x < 0 {
		x = -x
	}
	x = x - float64(int(x/(2*3.14159)))*2*3.14159

	result := x - (x*x*x)/6 + (x*x*x*x*x)/120
	return result
}
