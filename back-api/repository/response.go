package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateResponse creates a new response record
func (r *Repository) CreateResponse(ctx context.Context, resp *models.Response) error {
	if resp.ID == uuid.Nil {
		resp.ID = uuid.New()
	}

	query := `
		INSERT INTO responses (id, request_id, content, model, tokens_used, processing_time_ms, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`

	_, err := r.querier.Exec(ctx, query,
		resp.ID,
		resp.RequestID,
		resp.Content,
		resp.Model,
		resp.TokensUsed,
		resp.ProcessingTimeMs,
	)
	if err != nil {
		return fmt.Errorf("failed to create response: %w", err)
	}
	return nil
}

// GetResponseByRequestID retrieves the latest response for a request
func (r *Repository) GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error) {
	query := `
		SELECT id, request_id, content, model, tokens_used, processing_time_ms, created_at
		FROM responses
		WHERE request_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var resp models.Response
	err := r.querier.QueryRow(ctx, query, requestID).Scan(
		&resp.ID,
		&resp.RequestID,
		&resp.Content,
		&resp.Model,
		&resp.TokensUsed,
		&resp.ProcessingTimeMs,
		&resp.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No response yet is not an error
		}
		return nil, fmt.Errorf("failed to get response: %w", err)
	}

	return &resp, nil
}
