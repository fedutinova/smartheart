package handler

import (
	"net/http"

	"github.com/fedutinova/smartheart/back-api/service"
)

// H2ComparisonResult holds metrics for band vs OCR redaction comparison (H2 hypothesis).
type H2ComparisonResult struct {
	BandMetrics struct {
		RedactionMs      int     `json:"redaction_ms"`
		MaskedAreaRatio  float64 `json:"masked_area_ratio"`
		BoxesCount       int     `json:"boxes_count"`
		LeakRate         float64 `json:"leak_rate"` // residual / expected identifiers
	} `json:"band_metrics"`
	OCRMetrics struct {
		RedactionMs      int     `json:"redaction_ms"`
		MaskedAreaRatio  float64 `json:"masked_area_ratio"`
		BoxesCount       int     `json:"boxes_count"`
		LeakRate         float64 `json:"leak_rate"`
	} `json:"ocr_metrics"`
	ImageWidth  int    `json:"image_width"`
	ImageHeight int    `json:"image_height"`
	Message     string `json:"message"`
}

// CompareH2Redaction compares band vs OCR redaction on a single image.
// Used for H2 hypothesis verification: does OCR provide better trade-off
// between masked_area_ratio and leak_rate vs band masking?
func (h *ECGHandler) CompareH2Redaction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	// Extract image file
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer func() { _ = file.Close() }()

	// Parse expected identifiers (optional, for leak_rate calculation)
	expectedIdentifiers := r.FormValue("expected_identifiers") // comma-separated or JSON array
	_ = expectedIdentifiers // TODO: parse and use for leak_rate

	// Call service to compare redaction modes
	result, err := h.Service.CompareH2Redaction(r.Context(), service.UploadedFile{
		Reader:      file,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
