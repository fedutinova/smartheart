-- User feedback on the ML rhythm conclusion shown on the result page.
-- One row per (request_id, user_id); since each request has a single owner,
-- request_id alone is enough as a primary key. Re-voting upserts the rating.
CREATE TABLE IF NOT EXISTS rhythm_feedback (
    request_id UUID PRIMARY KEY REFERENCES requests(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    rating     TEXT NOT NULL CHECK (rating IN ('helpful', 'inaccurate', 'unclear')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rhythm_feedback_user    ON rhythm_feedback(user_id);
CREATE INDEX IF NOT EXISTS idx_rhythm_feedback_created ON rhythm_feedback(created_at);
CREATE INDEX IF NOT EXISTS idx_rhythm_feedback_rating  ON rhythm_feedback(rating);
