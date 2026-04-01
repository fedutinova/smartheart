package models

import (
	"time"

	"github.com/google/uuid"
)

// File represents an uploaded file associated with a request.
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
