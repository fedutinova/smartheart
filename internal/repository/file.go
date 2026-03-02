package repository

import (
	"context"

	"github.com/fedutinova/smartheart/internal/models"
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

	_, err := r.q.Exec(ctx, query,
		file.ID,
		file.RequestID,
		file.OriginalFilename,
		file.FileType,
		file.FileSize,
		file.S3Bucket,
		file.S3Key,
		file.S3URL,
	)
	return err
}

// GetFilesByRequestID retrieves all files for a request
func (r *Repository) GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at
		FROM files
		WHERE request_id = $1
		ORDER BY created_at
	`

	rows, err := r.q.Query(ctx, query, requestID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		files = append(files, file)
	}

	return files, rows.Err()
}

