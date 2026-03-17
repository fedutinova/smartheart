package repository

import (
	"context"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/models"
)

// CreateRAGFeedback inserts a new RAG feedback record.
func (r *Repository) CreateRAGFeedback(ctx context.Context, feedback *models.RAGFeedback) error {
	_, err := r.querier.Exec(ctx, `
		INSERT INTO rag_feedback (id, user_id, question, answer, rating)
		VALUES ($1, $2, $3, $4, $5)
	`, feedback.ID, feedback.UserID, feedback.Question, feedback.Answer, feedback.Rating)
	if err != nil {
		return fmt.Errorf("create rag feedback: %w", err)
	}
	return nil
}
