package models

import "encoding/json"

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
