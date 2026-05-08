package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/models"
)

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// CreateResponse creates a new response record.
func (r *Repository) CreateResponse(ctx context.Context, resp *models.Response) error {
	if resp.ID == uuid.Nil {
		resp.ID = uuid.New()
	}

	query := `
		INSERT INTO responses (
			id, request_id, content, model, tokens_used, processing_time_ms,
			cache_status, cache_entry_id, cache_trigram_similarity,
			cache_vector_similarity, cache_combined_similarity, cache_match_method,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
	`

	_, err := r.querier.Exec(ctx, query,
		resp.ID,
		resp.RequestID,
		resp.Content,
		resp.Model,
		resp.TokensUsed,
		resp.ProcessingTimeMs,
		nullString(resp.CacheStatus),
		resp.CacheEntryID,
		resp.CacheTrigramSimilarity,
		resp.CacheVectorSimilarity,
		resp.CacheCombinedSimilarity,
		nullString(resp.CacheMatchMethod),
	)
	if err != nil {
		return fmt.Errorf("failed to create response: %w", err)
	}
	return nil
}

// GetResponseByRequestID retrieves the latest response for a request.
func (r *Repository) GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error) {
	query := `
		SELECT id, request_id, content, model, tokens_used, processing_time_ms,
		       cache_status, cache_entry_id, cache_trigram_similarity,
		       cache_vector_similarity, cache_combined_similarity, cache_match_method,
		       created_at
		FROM responses
		WHERE request_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var resp models.Response
	var cacheStatus sql.NullString
	var cacheMatchMethod sql.NullString
	err := r.querier.QueryRow(ctx, query, requestID).Scan(
		&resp.ID,
		&resp.RequestID,
		&resp.Content,
		&resp.Model,
		&resp.TokensUsed,
		&resp.ProcessingTimeMs,
		&cacheStatus,
		&resp.CacheEntryID,
		&resp.CacheTrigramSimilarity,
		&resp.CacheVectorSimilarity,
		&resp.CacheCombinedSimilarity,
		&cacheMatchMethod,
		&resp.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // no response yet is a valid state, not an error
		}
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	if cacheStatus.Valid {
		resp.CacheStatus = cacheStatus.String
	}
	if cacheMatchMethod.Valid {
		resp.CacheMatchMethod = cacheMatchMethod.String
	}

	return &resp, nil
}
