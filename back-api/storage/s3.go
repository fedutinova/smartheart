package storage

import (
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
	"github.com/google/uuid"

	appconfig "github.com/fedutinova/smartheart/back-api/config"
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

	slog.InfoContext(ctx, "Initializing S3 storage",
		"endpoint", cfg.S3.Endpoint,
		"bucket", cfg.S3.Bucket,
		"region", cfg.S3.Region,
		"force_path_style", cfg.S3.ForcePathStyle)

	if cfg.S3.Endpoint != "" && (strings.Contains(cfg.S3.Endpoint, "localstack") || strings.Contains(cfg.S3.Endpoint, "localhost:4566") || strings.Contains(cfg.S3.Endpoint, "4566")) {
		slog.InfoContext(ctx, "Using LocalStack configuration")
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.S3.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.S3.AWSAccessKey,
				cfg.S3.AWSSecretKey,
				"",
			)),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config for LocalStack: %w", err)
		}

		client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3.Endpoint)
			o.UsePathStyle = cfg.S3.ForcePathStyle
		})

		return &S3Storage{
			client:   client,
			bucket:   cfg.S3.Bucket,
			endpoint: cfg.S3.Endpoint,
			region:   cfg.S3.Region,
		}, nil
	}

	if cfg.S3.AWSAccessKey != "" && cfg.S3.AWSSecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.S3.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.S3.AWSAccessKey,
				cfg.S3.AWSSecretKey,
				"",
			)),
		)
	} else {
		awsCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(cfg.S3.Region))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3Storage{
		client:   client,
		bucket:   cfg.S3.Bucket,
		endpoint: cfg.S3.Endpoint,
		region:   cfg.S3.Region,
	}, nil
}

func (s *S3Storage) UploadFile(ctx context.Context, filename string, content io.Reader, contentType string) (*UploadResult, error) {
	key := s.generateKey(filename)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        content,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	var url string
	if s.endpoint != "" {
		url = fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	} else {
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	}

	slog.InfoContext(ctx, "File uploaded to S3", "key", key, "bucket", s.bucket)

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

	slog.InfoContext(ctx, "File deleted from S3", "key", key, "bucket", s.bucket)
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

func (*S3Storage) generateKey(filename string) string {
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
