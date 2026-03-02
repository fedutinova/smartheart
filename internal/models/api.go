package models

import "github.com/google/uuid"

// APIError is a structured error response returned by API handlers.
type APIError struct {
	Error        string   `json:"error"`
	Details      any      `json:"details,omitempty"`
	UploadErrors []string `json:"upload_errors,omitempty"`
}

// RegisterResponse is returned on successful user registration.
type RegisterResponse struct {
	Message string    `json:"message"`
	UserID  uuid.UUID `json:"user_id"`
}

// SubmitGPTResponse is returned when a GPT analysis job is enqueued.
type SubmitGPTResponse struct {
	RequestID      uuid.UUID `json:"request_id"`
	JobID          uuid.UUID `json:"job_id"`
	Status         string    `json:"status"`
	FilesProcessed int       `json:"files_processed"`
	UploadErrors   []string  `json:"upload_errors,omitempty"`
}

// SubmitEKGResponse is returned when an EKG analysis job is enqueued.
type SubmitEKGResponse struct {
	JobID     string `json:"job_id"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}
