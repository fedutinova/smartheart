package storage

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	UploadFile(ctx context.Context, filename string, content io.Reader, contentType string) (*UploadResult, error)
	GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error)
	DeleteFile(ctx context.Context, key string) error
}

type UploadResult struct {
	Key string
	URL string
}
