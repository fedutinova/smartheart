package redaction

import (
	"io"
	"regexp"
)

// EvaluationResult represents metrics for PII leak detection.
type EvaluationResult struct {
	ResidualIdentifiers []string `json:"residual_identifiers"`
	LeakRate            float64  `json:"leak_rate"`
	OCRWords            []string `json:"ocr_words"`
}

// PII patterns to check for in redacted images.
var piiPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{1,2}[.\-\/]\d{1,2}[.\-\/]\d{2,4}\b`),                           // dates DD.MM.YYYY
	regexp.MustCompile(`\b\d{1,2}\s+(янв|фев|мар|апр|май|июн|июл|авг|сен|окт|ноя|дек)\w*\s+\d{4}\b`), // date words (Russian)
	regexp.MustCompile(`\b[А-Я]?\d{5,12}\b`),                                                 // patient IDs
	regexp.MustCompile(`\b\d{3}-\d{3}-\d{3}\s\d{2}\b`),                                       // СНИЛС
}

// EvaluateRedaction checks a redacted image for remaining PII.
// Currently uses simple regex patterns (no actual OCR on backend).
// TODO: Integrate Tesseract OCR to check actual image content.
func EvaluateRedaction(redactedImageBlob io.Reader, expectedIdentifiers []string) (EvaluationResult, error) {
	// Placeholder: in real implementation, would:
	// 1. Run Tesseract OCR on the image
	// 2. Extract all words
	// 3. Check for PII patterns in OCR output
	// 4. Fuzzy-match against expectedIdentifiers
	// 5. Calculate leakRate = found / expected

	// For now, return empty result (no leak detected)
	// This ensures the endpoint works while OCR integration is pending

	result := EvaluationResult{
		ResidualIdentifiers: []string{},
		LeakRate:            0.0,
		OCRWords:            []string{},
	}

	// If we had expectedIdentifiers, we'd normalize leakRate
	if len(expectedIdentifiers) > 0 {
		result.LeakRate = 0.0 / float64(len(expectedIdentifiers))
	}

	return result, nil
}

// DetectPIIPatterns searches text for PII using regex.
func DetectPIIPatterns(text string) []string {
	var found []string
	seen := make(map[string]bool)

	for _, pattern := range piiPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			if !seen[match] {
				found = append(found, match)
				seen[match] = true
			}
		}
	}

	return found
}
