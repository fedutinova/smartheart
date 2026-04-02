package repository

import (
	"context"
	"fmt"

	"github.com/fedutinova/smartheart/back-api/models"
)

// CreateRAGFeedback inserts or updates a RAG feedback record.
// If the same user already rated the same question+answer pair, the rating is updated.
func (r *Repository) CreateRAGFeedback(ctx context.Context, feedback *models.RAGFeedback) error {
	_, err := r.querier.Exec(ctx, `
		INSERT INTO rag_feedback (id, user_id, question, answer, rating)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, md5(question), md5(answer))
		DO UPDATE SET rating = EXCLUDED.rating, created_at = now()
	`, feedback.ID, feedback.UserID, feedback.Question, feedback.Answer, feedback.Rating)
	if err != nil {
		return fmt.Errorf("create rag feedback: %w", err)
	}
	return nil
}
