package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/fedutinova/smartheart/internal/config"
	"github.com/google/uuid"
)

type S3Storage struct {
	client   *s3.Client
	bucket   string
	endpoint string
	region   string
}

func NewS3Storage(ctx context.Context, cfg appconfig.Config) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	slog.Info("initializing S3 storage",
		"endpoint", cfg.S3Endpoint,
		"bucket", cfg.S3Bucket,
		"region", cfg.S3Region,
		"access_key", cfg.AWSAccessKey,
		"force_path_style", cfg.S3ForcePathStyle)

	if cfg.S3Endpoint != "" && (strings.Contains(cfg.S3Endpoint, "localstack") || strings.Contains(cfg.S3Endpoint, "localhost:4566") || strings.Contains(cfg.S3Endpoint, "4566")) {
		slog.Info("using LocalStack configuration")
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.S3Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKey,
				cfg.AWSSecretKey,
				"",
			)),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config for LocalStack: %w", err)
		}

		client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = cfg.S3ForcePathStyle
		})

		return &S3Storage{
			client:   client,
			bucket:   cfg.S3Bucket,
			endpoint: cfg.S3Endpoint,
			region:   cfg.S3Region,
		}, nil
	}

	if cfg.AWSAccessKey != "" && cfg.AWSSecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.S3Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKey,
				cfg.AWSSecretKey,
				"",
			)),
		)
	} else {
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.S3Region))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3Storage{
		client:   client,
		bucket:   cfg.S3Bucket,
		endpoint: cfg.S3Endpoint,
		region:   cfg.S3Region,
	}, nil
}

func (s *S3Storage) UploadFile(ctx context.Context, filename string, content io.Reader, contentType string) (*UploadResult, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	key := s.generateKey(filename)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	var url string
	if s.endpoint != "" && (strings.Contains(s.endpoint, "localstack") || strings.Contains(s.endpoint, "localhost:4566") || strings.Contains(s.endpoint, "4566")) {
		url = fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	} else if s.endpoint != "" {
		url = fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	} else {
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	}

	slog.Info("file uploaded to S3", "key", key, "bucket", s.bucket, "size", len(data))

	return &UploadResult{
		Key: key,
		URL: url,
	}, nil
}

func (s *S3Storage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL: %w", err)
	}

	return request.URL, nil
}

func (s *S3Storage) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	slog.Info("file deleted from S3", "key", key, "bucket", s.bucket)
	return nil
}

func (s *S3Storage) GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file from S3: %w", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	return result.Body, contentType, nil
}

func (s *S3Storage) generateKey(filename string) string {
	ext := filepath.Ext(filename)
	basename := strings.TrimSuffix(filepath.Base(filename), ext)

	safeBasename := strings.ReplaceAll(basename, " ", "_")
	safeBasename = strings.ReplaceAll(safeBasename, "/", "_")

	timestamp := time.Now().Format("2006/01/02")
	uniqueID := uuid.New().String()[:8]

	return fmt.Sprintf("uploads/%s/%s_%s%s", timestamp, safeBasename, uniqueID, ext)
}
