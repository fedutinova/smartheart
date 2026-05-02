package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/models"
)

// CreateECGChatMessage inserts a new chat message for an ECG request.
func (r *Repository) CreateECGChatMessage(ctx context.Context, msg *models.ECGChatMessage) error {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}

	var citationsJSON []byte
	if len(msg.Citations) > 0 {
		var err error
		citationsJSON, err = json.Marshal(msg.Citations)
		if err != nil {
			return fmt.Errorf("marshal citations: %w", err)
		}
	}

	query := `
		INSERT INTO ecg_chat_messages (id, request_id, user_id, role, content, citations, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING created_at
	`
	err := r.querier.QueryRow(ctx, query,
		msg.ID, msg.RequestID, msg.UserID, string(msg.Role), msg.Content, citationsJSON,
	).Scan(&msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("create ecg chat message: %w", err)
	}
	return nil
}

// GetECGChatMessages returns all messages for a given ECG request, ordered chronologically.
// userID is used to enforce ownership: only messages belonging to the same user are returned.
func (r *Repository) GetECGChatMessages(ctx context.Context, requestID, userID uuid.UUID) ([]models.ECGChatMessage, error) {
	query := `
		SELECT id, request_id, user_id, role, content, citations, created_at
		FROM ecg_chat_messages
		WHERE request_id = $1 AND user_id = $2
		ORDER BY created_at ASC
	`
	rows, err := r.querier.Query(ctx, query, requestID, userID)
	if err != nil {
		return nil, fmt.Errorf("query ecg chat messages: %w", err)
	}
	defer rows.Close()

	var messages []models.ECGChatMessage
	for rows.Next() {
		var msg models.ECGChatMessage
		var role string
		var citationsBytes []byte
		if err := rows.Scan(
			&msg.ID, &msg.RequestID, &msg.UserID, &role, &msg.Content, &citationsBytes, &msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ecg chat message: %w", err)
		}
		msg.Role = models.ECGChatRole(role)
		if len(citationsBytes) > 0 {
			if err := json.Unmarshal(citationsBytes, &msg.Citations); err != nil {
				return nil, fmt.Errorf("unmarshal citations for %s: %w", msg.ID, err)
			}
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ecg chat messages: %w", err)
	}
	return messages, nil
}
