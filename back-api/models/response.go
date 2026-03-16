package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Response represents an AI response to a request
type Response struct {
	ID               uuid.UUID `json:"id"`
	RequestID        uuid.UUID `json:"request_id"`
	Content          string    `json:"content"`
	Model            string    `json:"model,omitempty"`
	TokensUsed       int       `json:"tokens_used,omitempty"`
	ProcessingTimeMs int       `json:"processing_time_ms,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ResponseParsed is a Response with content parsed into a structured field.
type ResponseParsed struct {
	Response
	ContentParsed map[string]any `json:"content_parsed"`
}

// ParseContent converts Response to ResponseParsed by parsing Content as JSON.
func (r *Response) ParseContent() (*ResponseParsed, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(r.Content), &parsed); err != nil {
		return nil, err
	}
	return &ResponseParsed{
		Response:       *r,
		ContentParsed:  parsed,
	}, nil
}

// RequestParsed is a Request with a parsed response.
type RequestParsed struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"user_id,omitempty"`
	TextQuery *string         `json:"text_query,omitempty"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Files     []File          `json:"files,omitempty"`
	Response  *ResponseParsed `json:"response,omitempty"`
}

// WithParsedResponse converts Request to RequestParsed.
func (r *Request) WithParsedResponse(resp *ResponseParsed) *RequestParsed {
	return &RequestParsed{
		ID:        r.ID,
		UserID:    r.UserID,
		TextQuery: r.TextQuery,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Files:     r.Files,
		Response:  resp,
	}
}
