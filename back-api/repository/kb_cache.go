package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/fedutinova/smartheart/back-api/models"
)

// NormalizeQuestion lowercases and trims whitespace/punctuation for cache matching.
func NormalizeQuestion(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	// Collapse multiple spaces.
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	// Remove trailing punctuation.
	q = strings.TrimRight(q, "?!.,:;")
	return q
}

// FindCachedAnswer looks up a semantically similar question in kb_cache.
// Returns nil if no match above the similarity threshold is found.
func (r *Repository) FindCachedAnswer(ctx context.Context, question string, threshold float64) (*models.KBCacheEntry, error) {
	normalized := NormalizeQuestion(question)

	query := `
		SELECT id, question_normalized, answer, source_meta, hit_count,
		       similarity(question_normalized, $1) AS sim,
		       created_at, expires_at
		FROM kb_cache
		WHERE similarity(question_normalized, $1) >= $2
		  AND expires_at > now()
		ORDER BY sim DESC
		LIMIT 1
	`

	row := r.querier.QueryRow(ctx, query, normalized, threshold)

	var entry models.KBCacheEntry
	var sourceMeta *string
	err := row.Scan(
		&entry.ID,
		&entry.QuestionNormalized,
		&entry.Answer,
		&sourceMeta,
		&entry.HitCount,
		&entry.Similarity,
		&entry.CreatedAt,
		&entry.ExpiresAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("find cached answer: %w", err)
	}
	if sourceMeta != nil {
		entry.SourceMeta = *sourceMeta
	}

	// Increment hit counter asynchronously (best-effort).
	_, _ = r.querier.Exec(ctx,
		`UPDATE kb_cache SET hit_count = hit_count + 1 WHERE id = $1`,
		entry.ID,
	)

	return &entry, nil
}

// SaveCacheEntry stores a new question-answer pair in kb_cache.
func (r *Repository) SaveCacheEntry(ctx context.Context, question, answer, sourceMeta string) error {
	normalized := NormalizeQuestion(question)

	var meta *string
	if sourceMeta != "" {
		meta = &sourceMeta
	}
	_, err := r.querier.Exec(ctx, `
		INSERT INTO kb_cache (question_normalized, answer, source_meta)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, normalized, answer, meta)
	if err != nil {
		return fmt.Errorf("save cache entry: %w", err)
	}
	return nil
}
