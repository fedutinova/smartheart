package storage

import (
	"context"

	appconfig "github.com/fedutinova/smartheart/internal/config"
)

func NewStorage(ctx context.Context, cfg appconfig.Config) (Storage, error) {
	switch cfg.Storage.Mode {
	case "s3", "aws", "localstack":
		return NewS3Storage(ctx, cfg)
	case "local", "filesystem":
		return NewLocalStorage(cfg.Storage.LocalDir, cfg.Storage.LocalURL)
	default:
		return NewLocalStorage(cfg.Storage.LocalDir, cfg.Storage.LocalURL)
	}
}

func GetStorageType(cfg appconfig.Config) string {
	switch cfg.Storage.Mode {
	case "s3", "aws", "localstack":
		if cfg.S3.Endpoint != "" && (cfg.S3.Endpoint == "http://localhost:4566" || cfg.S3.Endpoint == "http://localstack:4566") {
			return "LocalStack S3"
		}
		return "AWS S3"
	case "local", "filesystem":
		return "Local Filesystem"
	default:
		return "Local Filesystem (default)"
	}
}
