package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/fedutinova/smartheart/internal/common"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateRequest creates a new request
func (r *Repository) CreateRequest(ctx context.Context, req *models.Request) error {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	query := `
		INSERT INTO requests (id, user_id, text_query, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`

	var textQuery sql.NullString
	if req.TextQuery != nil {
		textQuery = sql.NullString{String: *req.TextQuery, Valid: true}
	}

	_, err := r.q.Exec(ctx, query, req.ID, req.UserID, textQuery, req.Status)
	return err
}

// GetRequestByID retrieves a request by ID with files and response
func (r *Repository) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.Request, error) {
	query := `
		SELECT id, user_id, text_query, status, created_at, updated_at
		FROM requests
		WHERE id = $1
	`

	var req models.Request
	var textQuery sql.NullString

	err := r.q.QueryRow(ctx, query, id).Scan(
		&req.ID,
		&req.UserID,
		&textQuery,
		&req.Status,
		&req.CreatedAt,
		&req.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}

	if textQuery.Valid {
		req.TextQuery = &textQuery.String
	}

	files, err := r.GetFilesByRequestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}
	req.Files = files

	response, err := r.GetResponseByRequestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}
	req.Response = response

	return &req, nil
}

// GetRequestsByUserID retrieves all requests for a user
func (r *Repository) GetRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]models.Request, error) {
	query := `
		SELECT id, user_id, text_query, status, created_at, updated_at
		FROM requests
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.q.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.Request
	for rows.Next() {
		var req models.Request
		var textQuery sql.NullString

		err := rows.Scan(
			&req.ID,
			&req.UserID,
			&textQuery,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if textQuery.Valid {
			req.TextQuery = &textQuery.String
		}

		requests = append(requests, req)
	}

	return requests, rows.Err()
}

// UpdateRequestStatus updates the status of a request
func (r *Repository) UpdateRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error {
	query := `
		UPDATE requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.q.Exec(ctx, query, status, requestID)
	return err
}

