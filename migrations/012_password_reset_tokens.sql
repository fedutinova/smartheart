CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_password_reset_tokens_hash ON password_reset_tokens (token_hash) WHERE used_at IS NULL;
CREATE INDEX idx_password_reset_tokens_user ON password_reset_tokens (user_id) WHERE used_at IS NULL;
