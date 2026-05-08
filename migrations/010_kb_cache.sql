-- Hybrid cache for knowledge-base queries (H3 hypothesis).
-- Uses pg_trgm lexical similarity and pgvector embedding similarity.

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS kb_cache (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_normalized TEXT NOT NULL,
    question_embedding  vector(768), -- default intfloat/multilingual-e5-base dimension
    answer              TEXT NOT NULL,
    source_meta         JSONB,          -- optional: sources, model, etc.
    hit_count           INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days'
);

-- GIN trigram index for fast similarity search.
CREATE INDEX idx_kb_cache_question_trgm
    ON kb_cache USING gin (question_normalized gin_trgm_ops);

-- HNSW cosine index for semantic similarity search.
CREATE INDEX IF NOT EXISTS idx_kb_cache_question_embedding
    ON kb_cache USING hnsw (question_embedding vector_cosine_ops)
    WHERE question_embedding IS NOT NULL;

-- Index for TTL cleanup.
CREATE INDEX idx_kb_cache_expires ON kb_cache (expires_at);
