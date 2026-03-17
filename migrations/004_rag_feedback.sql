CREATE TABLE IF NOT EXISTS rag_feedback (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    question   TEXT NOT NULL,
    answer     TEXT NOT NULL,
    rating     SMALLINT NOT NULL CHECK (rating IN (-1, 1)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_rag_feedback_user ON rag_feedback(user_id);
CREATE INDEX idx_rag_feedback_created ON rag_feedback(created_at);
