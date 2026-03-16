package models

import (
	"encoding/json"
	"strings"
)

const EKGModelDirect = "ekg_direct_v2"

// EKGResponseContent is the typed structure stored in Response.Content
// for EKG analysis results.
type EKGResponseContent struct {
	AnalysisType            string  `json:"analysis_type"`
	Notes                   string  `json:"notes,omitempty"`
	Timestamp               string  `json:"timestamp"`
	JobID                   string  `json:"job_id"`
	GPTRequestID            string  `json:"gpt_request_id,omitempty"`
	GPTInterpretationStatus string  `json:"gpt_interpretation_status,omitempty"`
	GPTInterpretation       *string `json:"gpt_interpretation,omitempty"`
	GPTFullResponse         *string `json:"gpt_full_response,omitempty"`
}

// Marshal serializes to JSON string suitable for Response.Content.
func (e *EKGResponseContent) Marshal() (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

// ParseEKGContent tries to parse a Response.Content as EKGResponseContent.
// Returns nil, nil if analysis_type doesn't match.
func ParseEKGContent(content string) (*EKGResponseContent, error) {
	var ekg EKGResponseContent
	if err := json.Unmarshal([]byte(content), &ekg); err != nil {
		return nil, err
	}
	if ekg.AnalysisType != EKGModelDirect {
		return nil, nil
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
		"### Заключение\n",
		"### Заключение",
		"## Заключение\n",
		"## Заключение",
		"Заключение:\n",
		"Заключение:",
		"Заключение\n",
	}

	for _, marker := range markers {
		idx := strings.Index(response, marker)
		if idx != -1 {
			conclusion := strings.TrimSpace(response[idx+len(marker):])
			// Remove disclaimer at the end if present
			disclaimers := []string{
				"\n\nИнтерпретация носит информационный характер",
				"\nИнтерпретация носит информационный характер",
				"\n\nThis is for informational purposes",
			}
			for _, disclaimer := range disclaimers {
				if idx := strings.Index(conclusion, disclaimer); idx != -1 {
					conclusion = strings.TrimSpace(conclusion[:idx])
				}
			}
			return strings.TrimSpace(conclusion)
		}
	}

	// If no marker found, return full response (it should already be structured)
	return response
}
