package gpt

import (
	"strings"

	"github.com/google/uuid"
)

type GPTJobPayload struct {
	RequestID uuid.UUID `json:"request_id"`
	TextQuery string    `json:"text_query,omitempty"`
	FileKeys  []string  `json:"file_keys"`
	UserID    string    `json:"user_id,omitempty"`
}

// refusalPatterns are phrases that indicate GPT refused to process the request.
var refusalPatterns = []string{
	"i'm sorry",
	"i cannot",
	"can't assist",
	"unable to",
	"not able",
	"cannot analyze",
	"cannot recognize",
	"не могу",
	"извините",
	"не в состоянии",
}

// IsRefusal checks whether a GPT response contains a refusal message.
func IsRefusal(content string) bool {
	lower := strings.ToLower(content)
	for _, pattern := range refusalPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}