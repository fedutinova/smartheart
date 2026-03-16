package repository

import (
	"context"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/google/uuid"
)

// CreateFile creates a new file record
func (r *Repository) CreateFile(ctx context.Context, file *models.File) error {
	if file.ID == uuid.Nil {
		file.ID = uuid.New()
	}

	query := `
		INSERT INTO files (id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`

	_, err := r.querier.Exec(ctx, query,
		file.ID,
		file.RequestID,
		file.OriginalFilename,
		file.FileType,
		file.FileSize,
		file.S3Bucket,
		file.S3Key,
		file.S3URL,
	)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}

// GetFilesByRequestID retrieves all files for a request
func (r *Repository) GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at
		FROM files
		WHERE request_id = $1
		ORDER BY created_at
	`

	rows, err := r.querier.Query(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID,
			&file.RequestID,
			&file.OriginalFilename,
			&file.FileType,
			&file.FileSize,
			&file.S3Bucket,
			&file.S3Key,
			&file.S3URL,
			&file.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan file row: %w", err)
		}
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file rows: %w", err)
	}
	return files, nil
}
