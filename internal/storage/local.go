package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type LocalStorage struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}, nil
}

func (s *LocalStorage) UploadFile(ctx context.Context, filename string, content io.Reader, contentType string) (*UploadResult, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	key := s.generateKey(filename)
	filePath := filepath.Join(s.baseDir, key)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	url := fmt.Sprintf("%s/%s", s.baseURL, key)

	slog.Info("file uploaded to local storage", "key", key, "path", filePath, "size", len(data))

	return &UploadResult{
		Key: key,
		URL: url,
	}, nil
}

func (s *LocalStorage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	// for local storage, just return the direct URL (no expiration)
	url := fmt.Sprintf("%s/%s", s.baseURL, key)
	return url, nil
}

func (s *LocalStorage) DeleteFile(ctx context.Context, key string) error {
	filePath := filepath.Join(s.baseDir, key)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	slog.Info("file deleted from local storage", "key", key, "path", filePath)
	return nil
}

func (s *LocalStorage) GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	filePath := filepath.Join(s.baseDir, key)
	
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Error("file not found in local storage", "key", key, "path", filePath)
			return nil, "", fmt.Errorf("file not found: %s (path: %s)", key, filePath)
		}
		return nil, "", fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.Size() == 0 {
		return nil, "", fmt.Errorf("file is empty: %s", key)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}

	// Detect content type from extension
	contentType := "application/octet-stream"
	ext := filepath.Ext(key)
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".bmp":
		contentType = "image/bmp"
	case ".tiff", ".tif":
		contentType = "image/tiff"
	}

	slog.Debug("file opened from local storage",
		"key", key,
		"path", filePath,
		"size", fileInfo.Size(),
		"content_type", contentType)

	return file, contentType, nil
}

func (s *LocalStorage) generateKey(filename string) string {
	ext := filepath.Ext(filename)
	basename := filename[:len(filename)-len(ext)]

	safeBasename := filepath.Base(basename)

	timestamp := time.Now().Format("2006/01/02")
	uniqueID := uuid.New().String()[:8]

	return fmt.Sprintf("uploads/%s/%s_%s%s", timestamp, safeBasename, uniqueID, ext)
}
