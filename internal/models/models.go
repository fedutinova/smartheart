package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	Roles        []Role    `json:"roles,omitempty"`
}

type Role struct {
	ID          int          `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Description string       `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	Permissions []Permission `json:"permissions,omitempty"`
}

type Permission struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

type Request struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id,omitempty"`
	TextQuery *string   `json:"text_query,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Files     []File    `json:"files,omitempty"`
	Response  *Response `json:"response,omitempty"`
}

type File struct {
	ID               uuid.UUID `json:"id"`
	RequestID        uuid.UUID `json:"request_id"`
	OriginalFilename string    `json:"original_filename"`
	FileType         string    `json:"file_type,omitempty"`
	FileSize         int64     `json:"file_size,omitempty"`
	S3Bucket         string    `json:"s3_bucket,omitempty"`
	S3Key            string    `json:"s3_key"`
	S3URL            string    `json:"s3_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type Response struct {
	ID               uuid.UUID `json:"id"`
	RequestID        uuid.UUID `json:"request_id"`
	Content          string    `json:"content"`
	Model            string    `json:"model,omitempty"`
	TokensUsed       int       `json:"tokens_used,omitempty"`
	ProcessingTimeMs int       `json:"processing_time_ms,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)