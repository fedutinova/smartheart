package ekg

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"math"

	"gocv.io/x/gocv"
)

// EKGPreprocessor handles preprocessing of EKG images
type EKGPreprocessor struct {
	TargetWidth  int
	TargetHeight int
}

// PreprocessingResult contains the results of EKG image preprocessing
type PreprocessingResult struct {
	OriginalImage     gocv.Mat
	PreprocessedImage gocv.Mat
	SignalContour     []image.Point
	SignalLength      float64
	ProcessingSteps   []string
}

// NewEKGPreprocessor creates a new EKG preprocessor with default settings
func NewEKGPreprocessor() *EKGPreprocessor {
	return &EKGPreprocessor{
		TargetWidth:  800,
		TargetHeight: 600,
	}
}

// PreprocessImage performs complete EKG image preprocessing
func (p *EKGPreprocessor) PreprocessImage(ctx context.Context, imgData []byte) (*PreprocessingResult, error) {
	// Decode image from bytes
	img, err := gocv.IMDecode(imgData, gocv.IMReadColor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	defer img.Close()

	if img.Empty() {
		return nil, fmt.Errorf("empty image")
	}

	result := &PreprocessingResult{
		OriginalImage:   img.Clone(),
		ProcessingSteps: []string{},
	}

	// Step 1: Resize to fixed dimensions
	resized := p.resizeImage(img)
	result.ProcessingSteps = append(result.ProcessingSteps, "resized")
	slog.Debug("EKG preprocessing: resized image", "width", resized.Cols(), "height", resized.Rows())

	// Step 2: Convert to grayscale
	gray := p.convertToGrayscale(resized)
	result.ProcessingSteps = append(result.ProcessingSteps, "grayscale")
	slog.Debug("EKG preprocessing: converted to grayscale")

	// Step 3: Enhance contrast
	enhanced := p.enhanceContrast(gray)
	result.ProcessingSteps = append(result.ProcessingSteps, "contrast_enhanced")
	slog.Debug("EKG preprocessing: enhanced contrast")

	// Step 4: Apply adaptive threshold for binarization
	binary := p.adaptiveThreshold(enhanced)
	result.ProcessingSteps = append(result.ProcessingSteps, "binarized")
	slog.Debug("EKG preprocessing: applied adaptive threshold")

	// Step 5: Morphological operations
	morphed := p.morphologicalOperations(binary)
	result.ProcessingSteps = append(result.ProcessingSteps, "morphological_processed")
	slog.Debug("EKG preprocessing: applied morphological operations")

	// Step 6: Find the longest contour as EKG signal
	contour, length := p.findLongestContour(morphed)
	result.SignalContour = contour
	result.SignalLength = length
	result.ProcessingSteps = append(result.ProcessingSteps, "signal_extracted")
	slog.Debug("EKG preprocessing: extracted signal contour", "length", length, "points", len(contour))

	result.PreprocessedImage = morphed.Clone()

	// Clean up intermediate images
	resized.Close()
	gray.Close()
	enhanced.Close()
	binary.Close()
	morphed.Close()

	return result, nil
}

// resizeImage resizes image to target dimensions
func (p *EKGPreprocessor) resizeImage(img gocv.Mat) gocv.Mat {
	resized := gocv.NewMat()
	gocv.Resize(img, &resized, image.Point{X: p.TargetWidth, Y: p.TargetHeight}, 0, 0, gocv.InterpolationLinear)
	return resized
}

// convertToGrayscale converts color image to grayscale
func (p *EKGPreprocessor) convertToGrayscale(img gocv.Mat) gocv.Mat {
	gray := gocv.NewMat()
	gocv.CvtColor(img, &gray, gocv.ColorBGRToGray)
	return gray
}

// enhanceContrast applies histogram equalization for contrast enhancement
func (p *EKGPreprocessor) enhanceContrast(img gocv.Mat) gocv.Mat {
	enhanced := gocv.NewMat()
	gocv.EqualizeHist(img, &enhanced)
	return enhanced
}

// adaptiveThreshold applies adaptive threshold for binarization
func (p *EKGPreprocessor) adaptiveThreshold(img gocv.Mat) gocv.Mat {
	binary := gocv.NewMat()
	gocv.AdaptiveThreshold(img, &binary, 255, gocv.AdaptiveThresholdGaussian, gocv.ThresholdBinary, 11, 2)
	return binary
}

// morphologicalOperations applies erosion and dilation for noise reduction
func (p *EKGPreprocessor) morphologicalOperations(img gocv.Mat) gocv.Mat {
	// Create kernel for morphological operations
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Point{X: 3, Y: 3})
	defer kernel.Close()

	// Apply erosion to remove noise
	eroded := gocv.NewMat()
	gocv.Erode(img, &eroded, kernel)

	// Apply dilation to restore signal thickness
	dilated := gocv.NewMat()
	gocv.Dilate(eroded, &dilated, kernel)

	eroded.Close()
	return dilated
}

// findLongestContour finds the longest contour which represents the EKG signal
func (p *EKGPreprocessor) findLongestContour(img gocv.Mat) ([]image.Point, float64) {
	contours := gocv.FindContours(img, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	maxLength := 0.0
	longestIndex := -1

	// Find the contour with maximum arc length
	// Note: contours.At(i) returns a reference that belongs to contours
	// We should NOT close individual contours as they are managed by contours.Close()
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		arcLength := gocv.ArcLength(contour, true)
		if arcLength > maxLength {
			maxLength = arcLength
			longestIndex = i
		}
	}

	if longestIndex == -1 {
		slog.Warn("EKG preprocessing: no contours found")
		return []image.Point{}, 0.0
	}

	// Get the longest contour and convert to Go points
	// Must do this before contours.Close() is called via defer
	longestContour := contours.At(longestIndex)
	points := make([]image.Point, longestContour.Size())
	for i := 0; i < longestContour.Size(); i++ {
		point := longestContour.At(i)
		points[i] = image.Point{X: int(point.X), Y: int(point.Y)}
	}

	slog.Debug("EKG preprocessing: found longest contour",
		"contour_index", longestIndex,
		"total_contours", contours.Size(),
		"arc_length", maxLength,
		"points_count", len(points))

	// Note: longestContour is owned by contours and will be closed
	// when contours.Close() is called via defer. Do NOT close it explicitly.

	return points, maxLength
}

// ExtractSignalFeatures extracts features from the detected EKG signal
func (p *EKGPreprocessor) ExtractSignalFeatures(contour []image.Point) map[string]interface{} {
	if len(contour) < 2 {
		return map[string]interface{}{
			"error": "insufficient contour points",
		}
	}

	// Calculate basic statistics
	minX, maxX := contour[0].X, contour[0].X
	minY, maxY := contour[0].Y, contour[0].Y

	for _, point := range contour {
		if point.X < minX {
			minX = point.X
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}

	// Calculate signal amplitude range
	amplitudeRange := maxY - minY

	// Calculate signal width
	signalWidth := maxX - minX

	// Calculate average Y position (baseline)
	totalY := 0
	for _, point := range contour {
		totalY += point.Y
	}
	baseline := float64(totalY) / float64(len(contour))

	// Calculate signal variation (standard deviation from baseline)
	variance := 0.0
	for _, point := range contour {
		diff := float64(point.Y) - baseline
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(contour)))

	features := map[string]interface{}{
		"points_count":       len(contour),
		"signal_width":       signalWidth,
		"amplitude_range":    amplitudeRange,
		"baseline":           baseline,
		"standard_deviation": stdDev,
		"bounding_box": map[string]int{
			"min_x": minX,
			"max_x": maxX,
			"min_y": minY,
			"max_y": maxY,
		},
	}

	slog.Debug("EKG preprocessing: extracted signal features",
		"points", len(contour),
		"width", signalWidth,
		"amplitude", amplitudeRange,
		"baseline", baseline,
		"std_dev", stdDev)

	return features
}

// Close cleans up resources
func (result *PreprocessingResult) Close() {
	if !result.OriginalImage.Empty() {
		result.OriginalImage.Close()
	}
	if !result.PreprocessedImage.Empty() {
		result.PreprocessedImage.Close()
	}
}
