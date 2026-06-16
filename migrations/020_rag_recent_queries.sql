-- Per-user recent RAG questions, used to detect "re-asks": the same user
-- rephrasing a question within a short window (1 hour) because the first answer
-- did not satisfy them. When a re-ask is detected the KB cache is bypassed and a
-- fresh answer is generated, so the user is not handed back the same cached text.
--
-- Embeddings are stored so paraphrases ("the same thing in other words") are
-- caught semantically, not just by lexical overlap.
CREATE TABLE IF NOT EXISTS rag_recent_queries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             VARCHAR(255) NOT NULL,
    question_normalized TEXT NOT NULL,
    question_embedding  vector(768), -- intfloat/multilingual-e5-base dimension
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Lookups are always scoped to one user within a recent time window.
CREATE INDEX IF NOT EXISTS idx_rag_recent_queries_user_time
    ON rag_recent_queries (user_id, created_at DESC);
