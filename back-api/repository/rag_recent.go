package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HasRecentSimilarQuery reports whether the user already asked a semantically
// similar question within the given window. It backs the "re-ask" rule: if a
// user rephrases the same question shortly after receiving an answer, it signals
// dissatisfaction, so the KB cache must be bypassed and a fresh answer generated.
//
// Matching mirrors the cache lookup (hybrid trigram OR vector cosine) so a
// paraphrase is detected even when lexical overlap is low.
func (r *Repository) HasRecentSimilarQuery(
	ctx context.Context,
	userID uuid.UUID,
	question string,
	embedding []float64,
	within time.Duration,
	trigramThreshold,
	vectorThreshold float64,
) (bool, error) {
	normalized := NormalizeQuestion(question)
	cutoff := time.Now().Add(-within)

	var exists bool
	var err error
	if len(embedding) > 0 {
		err = r.querier.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM rag_recent_queries
				WHERE user_id = $1
				  AND created_at > $2
				  AND (
				      similarity(question_normalized, $3) >= $4
				      OR (
				          question_embedding IS NOT NULL
				          AND 1 - (question_embedding <=> $5::vector) >= $6
				      )
				  )
			)
		`, userID.String(), cutoff, normalized, trigramThreshold, formatVector(embedding), vectorThreshold).Scan(&exists)
	} else {
		err = r.querier.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM rag_recent_queries
				WHERE user_id = $1
				  AND created_at > $2
				  AND similarity(question_normalized, $3) >= $4
			)
		`, userID.String(), cutoff, normalized, trigramThreshold).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("check recent similar query: %w", err)
	}
	return exists, nil
}

// LogRecentQuery records a user's question (with embedding) so future re-asks
// can be detected, then prunes that user's rows older than one hour to keep the
// table bounded.
func (r *Repository) LogRecentQuery(
	ctx context.Context,
	userID uuid.UUID,
	question string,
	embedding []float64,
) error {
	normalized := NormalizeQuestion(question)
	var vector any
	if len(embedding) > 0 {
		vector = formatVector(embedding)
	}
	if _, err := r.querier.Exec(ctx, `
		INSERT INTO rag_recent_queries (user_id, question_normalized, question_embedding)
		VALUES ($1, $2, $3::vector)
	`, userID.String(), normalized, vector); err != nil {
		return fmt.Errorf("log recent query: %w", err)
	}

	// Best-effort pruning; failure here must not fail the request.
	_, _ = r.querier.Exec(ctx, `
		DELETE FROM rag_recent_queries
		WHERE user_id = $1 AND created_at < now() - INTERVAL '1 hour'
	`, userID.String())
	return nil
}
