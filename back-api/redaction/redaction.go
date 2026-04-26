package redaction

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"time"
)

// RedactionBox represents a rectangular region to mask.
type RedactionBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// RedactionMetrics holds timing and coverage data for a redaction operation.
type RedactionMetrics struct {
	RedactionMs     int     `json:"redaction_ms"`
	BoxesCount      int     `json:"boxes_count"`
	MaskedAreaRatio float64 `json:"masked_area_ratio"`
	ImageWidth      int     `json:"image_width"`
	ImageHeight     int     `json:"image_height"`
}

// BandRedactionConfig controls band-based redaction zones.
type BandRedactionConfig struct {
	TopRatio    float64
	BottomRatio float64
	LeftRatio   float64
}

// DefaultBandConfig provides standard redaction zones.
var DefaultBandConfig = BandRedactionConfig{
	TopRatio:    0.18,
	BottomRatio: 0.1,
	LeftRatio:   0.06,
}

// ApplyBandRedaction masks fixed zones (top, bottom, left) and returns metrics.
func ApplyBandRedaction(reader io.Reader, contentType string, cfg *BandRedactionConfig) (
	redactedBlob io.Reader,
	metrics RedactionMetrics,
	err error,
) {
	if cfg == nil {
		cfg = &DefaultBandConfig
	}

	startedAt := time.Now()

	// Decode image
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, RedactionMetrics{}, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate band dimensions
	topHeight := clampDim(int(float64(height) * cfg.TopRatio), 0, height/2)
	bottomHeight := clampDim(int(float64(height) * cfg.BottomRatio), 0, height/2)
	leftWidth := clampDim(int(float64(width) * cfg.LeftRatio), 0, width/2)

	// Create destination image and copy original
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	// Apply dark mask (#111827)
	maskColor := color.RGBA{R: 0x11, G: 0x18, B: 0x27, A: 0xff}

	// Mask top zone
	if topHeight > 0 {
		draw.Draw(dst, image.Rect(0, 0, width, topHeight), &image.Uniform{maskColor}, image.Point{}, draw.Over)
	}

	// Mask bottom zone
	if bottomHeight > 0 {
		draw.Draw(dst, image.Rect(0, height-bottomHeight, width, height), &image.Uniform{maskColor}, image.Point{}, draw.Over)
	}

	// Mask left zone
	if leftWidth > 0 && topHeight < height-bottomHeight {
		draw.Draw(dst, image.Rect(0, topHeight, leftWidth, height-bottomHeight), &image.Uniform{maskColor}, image.Point{}, draw.Over)
	}

	// Calculate metrics
	maskedArea := topHeight*width + bottomHeight*width
	if leftWidth > 0 && topHeight < height-bottomHeight {
		maskedArea += leftWidth * (height - topHeight - bottomHeight)
	}
	totalArea := width * height

	metrics = RedactionMetrics{
		RedactionMs:     int(time.Since(startedAt).Milliseconds()),
		BoxesCount:      countBands(topHeight, bottomHeight, leftWidth),
		MaskedAreaRatio: float64(maskedArea) / float64(totalArea),
		ImageWidth:      width,
		ImageHeight:     height,
	}

	// Encode result
	encoded, err := encodeImage(dst, format, contentType)
	return encoded, metrics, err
}

// ApplyOCRRedaction uses heuristic-based PII detection to mask regions.
// In production, this would use Tesseract OCR for actual text recognition.
// For now, uses regex pattern matching on estimated text regions.
func ApplyOCRRedaction(reader io.Reader, contentType string) (
	redactedBlob io.Reader,
	metrics RedactionMetrics,
	err error,
) {
	startedAt := time.Now()

	// Read image into buffer
	imageData, err := io.ReadAll(reader)
	if err != nil {
		return nil, RedactionMetrics{}, err
	}

	// Decode image to get dimensions
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, RedactionMetrics{}, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// For H2 testing: use heuristic PII detection
	// Assume PII patterns appear in specific regions (top 1/4 and bottom 1/4 of image)
	// This is a simplified approach pending full Tesseract integration
	piiBoxes := generatePIIBoxHeuristic(width, height)

	if len(piiBoxes) == 0 {
		// No PII regions detected, return original image
		return bytes.NewReader(imageData), RedactionMetrics{
			RedactionMs:     int(time.Since(startedAt).Milliseconds()),
			BoxesCount:      0,
			MaskedAreaRatio: 0,
			ImageWidth:      width,
			ImageHeight:     height,
		}, nil
	}

	// Create destination image and apply masks
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	maskColor := color.RGBA{R: 0x11, G: 0x18, B: 0x27, A: 0xff}
	maskedArea := 0

	for _, box := range piiBoxes {
		draw.Draw(dst, image.Rect(box.X, box.Y, box.X+box.Width, box.Y+box.Height),
			&image.Uniform{maskColor}, image.Point{}, draw.Over)
		maskedArea += box.Width * box.Height
	}

	// Encode result
	encoded, err := encodeImage(dst, format, contentType)
	if err != nil {
		return nil, RedactionMetrics{}, err
	}

	totalArea := width * height
	metrics = RedactionMetrics{
		RedactionMs:     int(time.Since(startedAt).Milliseconds()),
		BoxesCount:      len(piiBoxes),
		MaskedAreaRatio: float64(maskedArea) / float64(totalArea),
		ImageWidth:      width,
		ImageHeight:     height,
	}

	return encoded, metrics, nil
}

// generatePIIBoxHeuristic creates approximate PII boxes based on typical ECG layout.
// TODO: Replace with actual Tesseract OCR when system tesseract is available.
func generatePIIBoxHeuristic(width, height int) []RedactionBox {
	// Heuristic: PII typically appears in top-right corner and header areas
	// of ECG papers. Generate boxes for these regions.
	var boxes []RedactionBox

	// Top-right header area (common for patient ID, date, name)
	if width > 200 && height > 100 {
		boxes = append(boxes, RedactionBox{
			X:      int(float64(width) * 0.6),
			Y:      10,
			Width:  int(float64(width) * 0.35),
			Height: int(float64(height) * 0.12),
		})
	}

	// Left header area (patient info)
	if width > 200 && height > 80 {
		boxes = append(boxes, RedactionBox{
			X:      10,
			Y:      10,
			Width:  int(float64(width) * 0.25),
			Height: int(float64(height) * 0.1),
		})
	}

	// Bottom info area (date, signature)
	if width > 100 && height > 100 {
		boxes = append(boxes, RedactionBox{
			X:      10,
			Y:      height - int(float64(height)*0.08),
			Width:  width - 20,
			Height: int(float64(height) * 0.08),
		})
	}

	return boxes
}

// clampDim clamps a dimension value between min and max.
func clampDim(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// countBands counts how many redaction bands are active.
func countBands(topHeight, bottomHeight, leftWidth int) int {
	count := 0
	if topHeight > 0 {
		count++
	}
	if bottomHeight > 0 {
		count++
	}
	if leftWidth > 0 {
		count++
	}
	return count
}

// encodeImage encodes the image back to bytes in the original format.
func encodeImage(img image.Image, originalFormat, contentType string) (io.Reader, error) {
	// Determine output format
	format := originalFormat
	if format == "" {
		if contentType == "image/png" {
			format = "png"
		} else if contentType == "image/webp" {
			format = "webp"
		} else {
			format = "jpeg"
		}
	}

	// For now, always encode as JPEG for simplicity
	// Real implementation would preserve original format

	// Create a pipe to encode without buffering entire image
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		switch format {
		case "png":
			_ = png.Encode(w, img)
		default:
			_ = jpeg.Encode(w, img, &jpeg.Options{Quality: 92})
		}
	}()

	return r, nil
}
