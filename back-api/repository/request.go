package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

// CreateRequest creates a new request.
func (r *Repository) CreateRequest(ctx context.Context, req *models.Request) error {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	clientMeta, err := marshalClientMeta(req.ClientMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal client meta: %w", err)
	}

	query := `
		INSERT INTO requests (id, user_id, text_query, status, client_meta, ecg_age, ecg_sex, ecg_paper_speed_mms, ecg_mm_per_mv_limb, ecg_mm_per_mv_chest, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`

	_, err = r.querier.Exec(ctx, query, req.ID, req.UserID, req.TextQuery, req.Status, clientMeta,
		req.ECGAge, req.ECGSex, req.ECGPaperSpeedMMS, req.ECGMmPerMvLimb, req.ECGMmPerMvChest)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	return nil
}

// GetRequestByID retrieves a request by ID with files and response using a single query
// for the request + response, and a separate query for files.
func (r *Repository) GetRequestByID(ctx context.Context, id uuid.UUID) (*models.Request, error) {
	query := `
		SELECT r.id, r.user_id, r.text_query, r.status, r.created_at, r.updated_at, r.client_meta,
		       r.ecg_age, r.ecg_sex, r.ecg_paper_speed_mms, r.ecg_mm_per_mv_limb, r.ecg_mm_per_mv_chest,
		       resp.id, resp.request_id, resp.content, resp.model,
		       resp.tokens_used, resp.processing_time_ms, resp.created_at
		FROM requests r
		LEFT JOIN LATERAL (
			SELECT * FROM responses WHERE request_id = r.id ORDER BY created_at DESC LIMIT 1
		) resp ON true
		WHERE r.id = $1
	`

	var req models.Request

	// Response columns (nullable because of LEFT JOIN)
	var respID, respReqID *uuid.UUID
	var respContent, respModel *string
	var respTokens, respTimeMs *int
	var respCreatedAt *time.Time
	var clientMetaBytes []byte

	err := r.querier.QueryRow(ctx, query, id).Scan(
		&req.ID, &req.UserID, &req.TextQuery, &req.Status, &req.CreatedAt, &req.UpdatedAt, &clientMetaBytes,
		&req.ECGAge, &req.ECGSex, &req.ECGPaperSpeedMMS, &req.ECGMmPerMvLimb, &req.ECGMmPerMvChest,
		&respID, &respReqID, &respContent, &respModel,
		&respTokens, &respTimeMs, &respCreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrRequestNotFound
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}
	if req.ClientMeta, err = unmarshalClientMeta(clientMetaBytes); err != nil {
		return nil, fmt.Errorf("failed to decode client meta: %w", err)
	}

	// Assemble response if the JOIN returned data
	if respID != nil {
		resp := &models.Response{
			ID:               *respID,
			RequestID:        *respReqID,
			Content:          *respContent,
			Model:            *respModel,
			TokensUsed:       *respTokens,
			ProcessingTimeMs: *respTimeMs,
		}
		if respCreatedAt != nil {
			resp.CreatedAt = *respCreatedAt
		}
		req.Response = resp
	}

	// Files still need a separate query (one-to-many)
	files, err := r.GetFilesByRequestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get files: %w", err)
	}
	req.Files = files

	return &req, nil
}

// GetRequestsByUserID retrieves requests for a user with pagination.
func (r *Repository) GetRequestsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Request, error) {
	query := `
		SELECT id, user_id, text_query, status, created_at, updated_at, client_meta,
		       ecg_age, ecg_sex, ecg_paper_speed_mms, ecg_mm_per_mv_limb, ecg_mm_per_mv_chest
		FROM requests
		WHERE user_id = $1 AND ecg_paper_speed_mms IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.querier.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}
	defer rows.Close()

	var requests []models.Request
	for rows.Next() {
		var req models.Request
		var clientMetaBytes []byte
		err := rows.Scan(
			&req.ID,
			&req.UserID,
			&req.TextQuery,
			&req.Status,
			&req.CreatedAt,
			&req.UpdatedAt,
			&clientMetaBytes,
			&req.ECGAge,
			&req.ECGSex,
			&req.ECGPaperSpeedMMS,
			&req.ECGMmPerMvLimb,
			&req.ECGMmPerMvChest,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan request: %w", err)
		}
		req.ClientMeta, err = unmarshalClientMeta(clientMetaBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decode request client meta: %w", err)
		}

		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request rows: %w", err)
	}
	return requests, nil
}

// GetRecentRequestsWithResponses retrieves recent requests for a user with their
// latest response eagerly loaded, avoiding N+1 queries in fallback logic.
func (r *Repository) GetRecentRequestsWithResponses(ctx context.Context, userID uuid.UUID, limit int) ([]models.Request, error) {
	query := `
		SELECT r.id, r.user_id, r.text_query, r.status, r.created_at, r.updated_at, r.client_meta,
		       r.ecg_age, r.ecg_sex, r.ecg_paper_speed_mms, r.ecg_mm_per_mv_limb, r.ecg_mm_per_mv_chest,
		       resp.id, resp.request_id, resp.content, resp.model,
		       resp.tokens_used, resp.processing_time_ms, resp.created_at
		FROM requests r
		LEFT JOIN LATERAL (
			SELECT * FROM responses WHERE request_id = r.id ORDER BY created_at DESC LIMIT 1
		) resp ON true
		WHERE r.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2
	`

	rows, err := r.querier.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests with responses: %w", err)
	}
	defer rows.Close()

	var requests []models.Request
	for rows.Next() {
		var req models.Request
		var respID, respReqID *uuid.UUID
		var respContent, respModel *string
		var respTokens, respTimeMs *int
		var respCreatedAt *time.Time
		var clientMetaBytes []byte

		err := rows.Scan(
			&req.ID, &req.UserID, &req.TextQuery, &req.Status, &req.CreatedAt, &req.UpdatedAt, &clientMetaBytes,
			&req.ECGAge, &req.ECGSex, &req.ECGPaperSpeedMMS, &req.ECGMmPerMvLimb, &req.ECGMmPerMvChest,
			&respID, &respReqID, &respContent, &respModel,
			&respTokens, &respTimeMs, &respCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan request with response: %w", err)
		}
		req.ClientMeta, err = unmarshalClientMeta(clientMetaBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decode request client meta: %w", err)
		}

		if respID != nil {
			resp := &models.Response{
				ID:               *respID,
				RequestID:        *respReqID,
				Content:          *respContent,
				Model:            *respModel,
				TokensUsed:       *respTokens,
				ProcessingTimeMs: *respTimeMs,
			}
			if respCreatedAt != nil {
				resp.CreatedAt = *respCreatedAt
			}
			req.Response = resp
		}

		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request-with-response rows: %w", err)
	}
	return requests, nil
}

// CountRequestsByUserID returns the total number of requests for a user.
func (r *Repository) CountRequestsByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.querier.QueryRow(ctx, `SELECT COUNT(*) FROM requests WHERE user_id = $1 AND ecg_paper_speed_mms IS NOT NULL`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count requests: %w", err)
	}
	return count, nil
}

// UpdateRequestStatus updates the status of a request.
// Returns an error if status is not a known RequestStatus value.
func (r *Repository) UpdateRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error {
	if !models.ValidRequestStatus(status) {
		return fmt.Errorf("invalid request status: %q", status)
	}

	query := `
		UPDATE requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	tag, err := r.querier.Exec(ctx, query, status, requestID)
	if err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrRequestNotFound
	}
	return nil
}

func marshalClientMeta(meta *models.RequestClientMeta) ([]byte, error) {
	if meta == nil {
		return nil, nil
	}
	return json.Marshal(meta)
}

func unmarshalClientMeta(raw []byte) (*models.RequestClientMeta, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var meta models.RequestClientMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
