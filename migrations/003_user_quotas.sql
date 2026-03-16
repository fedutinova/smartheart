-- User daily quota tracking.
-- Counts submissions per user per day for rate limiting.

CREATE TABLE IF NOT EXISTS user_daily_usage (
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    usage_date DATE        NOT NULL DEFAULT CURRENT_DATE,
    count      INT         NOT NULL DEFAULT 1,
    PRIMARY KEY (user_id, usage_date)
);

CREATE INDEX idx_user_daily_usage_date ON user_daily_usage (usage_date);
