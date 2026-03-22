-- Payment tracking for YooKassa integration.
-- Stores payment records and purchased analysis packs.

CREATE TABLE IF NOT EXISTS payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    yookassa_id     TEXT        NOT NULL UNIQUE,       -- YooKassa payment ID
    status          TEXT        NOT NULL DEFAULT 'pending',  -- pending, succeeded, canceled
    amount_kopecks  INT         NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    analyses_count  INT         NOT NULL DEFAULT 1,    -- how many analyses this payment buys
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ
);

CREATE INDEX idx_payments_user    ON payments(user_id);
CREATE INDEX idx_payments_status  ON payments(status);
CREATE INDEX idx_payments_yookassa ON payments(yookassa_id);

-- Tracks purchased (paid) analyses remaining per user.
-- Decremented when user submits beyond free quota.
ALTER TABLE users ADD COLUMN IF NOT EXISTS paid_analyses_remaining INT NOT NULL DEFAULT 0;
