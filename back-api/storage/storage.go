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
	GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) // Returns reader, contentType, error
}

type UploadResult struct {
	Key string
	URL string
}
