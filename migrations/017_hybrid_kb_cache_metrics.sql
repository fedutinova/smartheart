CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE kb_cache
    ADD COLUMN IF NOT EXISTS question_embedding vector(768); -- default intfloat/multilingual-e5-base dimension

CREATE INDEX IF NOT EXISTS idx_kb_cache_question_embedding
    ON kb_cache USING hnsw (question_embedding vector_cosine_ops)
    WHERE question_embedding IS NOT NULL;

ALTER TABLE responses
    ADD COLUMN IF NOT EXISTS cache_status TEXT,
    ADD COLUMN IF NOT EXISTS cache_entry_id UUID REFERENCES kb_cache(id),
    ADD COLUMN IF NOT EXISTS cache_trigram_similarity DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS cache_vector_similarity DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS cache_combined_similarity DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS cache_match_method TEXT;

ALTER TABLE responses
    DROP CONSTRAINT IF EXISTS responses_cache_status_check,
    ADD CONSTRAINT responses_cache_status_check
        CHECK (cache_status IS NULL OR cache_status IN ('HIT', 'MISS'));

CREATE INDEX IF NOT EXISTS idx_responses_cache_status
    ON responses(cache_status)
    WHERE cache_status IS NOT NULL;
