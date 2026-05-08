package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

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

// FindCachedAnswer looks up a similar question in kb_cache using pg_trgm and,
// when available, pgvector cosine similarity. Returns nil if no match above
// either threshold is found.
func (r *Repository) FindCachedAnswer(
	ctx context.Context,
	question string,
	embedding []float64,
	trigramThreshold,
	vectorThreshold float64,
) (*models.KBCacheEntry, error) {
	normalized := NormalizeQuestion(question)

	var row pgx.Row
	if len(embedding) > 0 {
		query := `
			WITH candidates AS (
				SELECT id, question_normalized, answer, source_meta, hit_count,
				       similarity(question_normalized, $1) AS trigram_similarity,
				       CASE
				           WHEN question_embedding IS NULL THEN NULL
				           ELSE 1 - (question_embedding <=> $2::vector)
				       END AS vector_similarity,
				       created_at, expires_at
				FROM kb_cache
				WHERE expires_at > now()
				  AND (
				      similarity(question_normalized, $1) >= $3
				      OR (
				          question_embedding IS NOT NULL
				          AND 1 - (question_embedding <=> $2::vector) >= $4
				      )
				  )
			)
			SELECT id, question_normalized, answer, source_meta, hit_count,
			       trigram_similarity, vector_similarity,
			       GREATEST(trigram_similarity, COALESCE(vector_similarity, 0)) AS combined_similarity,
			       CASE
			           WHEN COALESCE(vector_similarity, 0) >= $4 AND vector_similarity >= trigram_similarity THEN 'vector'
			           WHEN trigram_similarity >= $3 THEN 'trigram'
			           ELSE 'combined'
			       END AS match_method,
			       created_at, expires_at
			FROM candidates
			ORDER BY combined_similarity DESC
			LIMIT 1
		`
		row = r.querier.QueryRow(ctx, query, normalized, formatVector(embedding), trigramThreshold, vectorThreshold)
	} else {
		query := `
			SELECT id, question_normalized, answer, source_meta, hit_count,
			       similarity(question_normalized, $1) AS trigram_similarity,
			       NULL::double precision AS vector_similarity,
			       similarity(question_normalized, $1) AS combined_similarity,
			       'trigram' AS match_method,
			       created_at, expires_at
			FROM kb_cache
			WHERE similarity(question_normalized, $1) >= $2
			  AND expires_at > now()
			ORDER BY combined_similarity DESC
			LIMIT 1
		`
		row = r.querier.QueryRow(ctx, query, normalized, trigramThreshold)
	}

	var entry models.KBCacheEntry
	var sourceMeta *string
	err := row.Scan(
		&entry.ID,
		&entry.QuestionNormalized,
		&entry.Answer,
		&sourceMeta,
		&entry.HitCount,
		&entry.TrigramSimilarity,
		&entry.VectorSimilarity,
		&entry.CombinedSimilarity,
		&entry.MatchMethod,
		&entry.CreatedAt,
		&entry.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // no cache hit is not an error
		}
		return nil, fmt.Errorf("find cached answer: %w", err)
	}
	if sourceMeta != nil {
		entry.SourceMeta = *sourceMeta
	}
	entry.Similarity = entry.CombinedSimilarity

	// Increment hit counter asynchronously (best-effort).
	_, _ = r.querier.Exec(ctx,
		`UPDATE kb_cache SET hit_count = hit_count + 1 WHERE id = $1`,
		entry.ID,
	)

	return &entry, nil
}

// SaveCacheEntry stores a new question-answer pair in kb_cache.
func (r *Repository) SaveCacheEntry(ctx context.Context, question string, embedding []float64, answer, sourceMeta string) error {
	normalized := NormalizeQuestion(question)

	var meta *string
	if sourceMeta != "" {
		meta = &sourceMeta
	}
	var vector any
	if len(embedding) > 0 {
		vector = formatVector(embedding)
	}
	_, err := r.querier.Exec(ctx, `
		INSERT INTO kb_cache (question_normalized, question_embedding, answer, source_meta)
		VALUES ($1, $2::vector, $3, $4)
		ON CONFLICT DO NOTHING
	`, normalized, vector, answer, meta)
	if err != nil {
		return fmt.Errorf("save cache entry: %w", err)
	}
	return nil
}

func formatVector(values []float64) string {
	var b strings.Builder
	b.Grow(len(values) * 10)
	b.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}
