package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LocalStorage struct {
	baseDir string
	baseURL string
}

func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}, nil
}

func (s *LocalStorage) UploadFile(_ context.Context, filename string, content io.Reader, _ string) (*UploadResult, error) {
	key := s.generateKey(filename)
	filePath := filepath.Join(s.baseDir, key)

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	written, err := io.Copy(f, content)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filePath) // clean up incomplete file
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	url := fmt.Sprintf("%s/%s", s.baseURL, key)

	slog.Info("file uploaded to local storage", "key", key, "path", filePath, "size", written)

	return &UploadResult{
		Key: key,
		URL: url,
	}, nil
}

func (*LocalStorage) GetPresignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", errors.New("presigned URLs not supported for local storage")
}

// safePath resolves the key to an absolute path inside baseDir, rejecting
// any traversal that escapes the storage root (symlinks included).
func (s *LocalStorage) safePath(key string) (string, error) {
	baseDir, err := filepath.Abs(filepath.Clean(s.baseDir))
	if err != nil {
		return "", fmt.Errorf("invalid base dir: %w", err)
	}

	cleaned := filepath.Clean("/" + key)
	full := filepath.Join(baseDir, cleaned)

	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(resolved, baseDir+string(os.PathSeparator)) && resolved != baseDir {
		return "", errors.New("path escapes storage root")
	}
	return resolved, nil
}

func (s *LocalStorage) DeleteFile(_ context.Context, key string) error {
	filePath, err := s.safePath(key)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	slog.Info("file deleted from local storage", "key", key, "path", filePath)
	return nil
}

func (s *LocalStorage) GetFile(_ context.Context, key string) (io.ReadCloser, string, error) {
	filePath, err := s.safePath(key)
	if err != nil {
		return nil, "", fmt.Errorf("invalid key: %w", err)
	}

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

func (*LocalStorage) generateKey(filename string) string {
	// filepath.Base strips directory components including ".." traversal
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	basename := strings.TrimSuffix(base, ext)

	// Remove dangerous characters: null bytes, backslashes, path separators
	r := strings.NewReplacer(
		"\x00", "",
		"\\", "_",
		"/", "_",
		" ", "_",
		"..", "_",
	)
	safeBasename := r.Replace(basename)
	safeExt := r.Replace(ext)

	if safeBasename == "" || safeBasename == "." {
		safeBasename = "file"
	}

	timestamp := time.Now().Format("2006/01/02")
	uniqueID := uuid.New().String()[:8]

	return fmt.Sprintf("uploads/%s/%s_%s%s", timestamp, safeBasename, uniqueID, safeExt)
}
