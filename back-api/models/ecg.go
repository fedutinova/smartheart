package models

import (
	"encoding/json"
	"strings"
)

const (
	ECGModelDirect     = "ekg_direct_v2"
	ECGModelStructured = "ekg_structured_v1"
)

// ECGResponseContent is the typed structure stored in Response.Content
// for EKG analysis results.
type ECGResponseContent struct {
	AnalysisType            string               `json:"analysis_type"`
	Notes                   string               `json:"notes,omitempty"`
	Timestamp               string               `json:"timestamp"`
	JobID                   string               `json:"job_id"`
	GPTRequestID            string               `json:"gpt_request_id,omitempty"`
	GPTInterpretationStatus string               `json:"gpt_interpretation_status,omitempty"`
	GPTInterpretation       *string              `json:"gpt_interpretation,omitempty"`
	GPTFullResponse         *string              `json:"gpt_full_response,omitempty"`
	StructuredResult        *ECGStructuredResult `json:"structured_result,omitempty"`
	// RhythmResult is the rhythm-classifier block produced by cv_service.
	// Nil for legacy responses and when CV inference was unavailable; the
	// frontend treats it as optional.
	RhythmResult *ECGRhythmResult `json:"rhythm_result,omitempty"`
}

// Marshal serializes to JSON string suitable for Response.Content.
func (e *ECGResponseContent) Marshal() (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

// ParseECGContent tries to parse a Response.Content as ECGResponseContent.
// Returns nil, nil if analysis_type doesn't match.
func ParseECGContent(content string) (*ECGResponseContent, error) {
	var ekg ECGResponseContent
	if err := json.Unmarshal([]byte(content), &ekg); err != nil {
		return nil, err
	}
	if ekg.AnalysisType != ECGModelDirect && ekg.AnalysisType != ECGModelStructured {
		return nil, nil //nolint:nilnil // nil,nil signals "not this type" without error
	}
	return &ekg, nil
}

// ExtractConclusion extracts structured conclusion from GPT response.
// Returns the full response if it's already structured with bullet points or numbered list.
func ExtractConclusion(gptResponse string) string {
	response := strings.TrimSpace(gptResponse)

	// Check if response is already structured (starts with number or bullet point)
	if strings.HasPrefix(response, "1.") || strings.HasPrefix(response, "•") ||
		strings.HasPrefix(response, "-") || strings.HasPrefix(response, "*") {
		return response
	}

	// Try to find conclusion section
	markers := []string{
		"### Итог\n",
		"### Итог",
		"## Итог\n",
		"## Итог",
		"### Заключение\n",
		"### Заключение",
		"## Заключение\n",
		"## Заключение",
		"Заключение:\n",
		"Заключение:",
		"Заключение\n",
		"Итог:\n",
		"Итог:",
		"Итог\n",
	}

	for _, marker := range markers {
		idx := strings.Index(response, marker)
		if idx != -1 {
			conclusion := strings.TrimSpace(response[idx+len(marker):])
			if nextHeader := strings.Index(conclusion, "\n## "); nextHeader != -1 {
				conclusion = strings.TrimSpace(conclusion[:nextHeader])
			}
			// Remove disclaimer at the end if present
			disclaimers := []string{
				"\n\nИнтерпретация носит информационный характер",
				"\nИнтерпретация носит информационный характер",
				"\n\nThis is for informational purposes",
			}
			for _, disclaimer := range disclaimers {
				if discIdx := strings.Index(conclusion, disclaimer); discIdx != -1 {
					conclusion = strings.TrimSpace(conclusion[:discIdx])
				}
			}
			return strings.TrimSpace(conclusion)
		}
	}

	// If no marker found, return full response (it should already be structured)
	return response
}

// ecgDisclaimerMarker is a stable substring used to detect an already-present
// disclaimer so WithECGDisclaimer stays idempotent.
const ecgDisclaimerMarker = "требует обязательной проверки врачом"

// ECGInterpretationDisclaimer is appended to every GPT-produced ECG
// interpretation shown to the user, making explicit that it is not a medical
// conclusion and must be reviewed by a doctor.
const ECGInterpretationDisclaimer = "\n\n---\n\n> ⚠️ **Важно:** это автоматическая интерпретация ЭКГ, а не медицинское заключение. " +
	"Она может содержать ошибки и **требует обязательной проверки врачом**. " +
	"Не используйте результат для самостоятельной постановки диагноза или лечения."

// WithECGDisclaimer appends the medical disclaimer to a non-empty interpretation.
// It is idempotent: the disclaimer is not added if it is already present.
func WithECGDisclaimer(md string) string {
	trimmed := strings.TrimRight(md, " \t\n")
	if trimmed == "" {
		return md
	}
	if strings.Contains(trimmed, ecgDisclaimerMarker) {
		return trimmed
	}
	return trimmed + ECGInterpretationDisclaimer
}
