package storage

import (
	"context"

	appconfig "github.com/fedutinova/smartheart/internal/config"
)

func NewStorage(ctx context.Context, cfg appconfig.Config) (Storage, error) {
	switch cfg.StorageMode {
	case "s3", "aws", "localstack":
		return NewS3Storage(ctx, cfg)
	case "local", "filesystem":
		return NewLocalStorage(cfg.LocalStorageDir, cfg.LocalStorageURL)
	default:
		return NewLocalStorage(cfg.LocalStorageDir, cfg.LocalStorageURL)
	}
}

func GetStorageType(cfg appconfig.Config) string {
	switch cfg.StorageMode {
	case "s3", "aws", "localstack":
		if cfg.S3Endpoint != "" && (cfg.S3Endpoint == "http://localhost:4566" || cfg.S3Endpoint == "http://localstack:4566") {
			return "LocalStack S3"
		}
		return "AWS S3"
	case "local", "filesystem":
		return "Local Filesystem"
	default:
		return "Local Filesystem (default)"
	}
}
