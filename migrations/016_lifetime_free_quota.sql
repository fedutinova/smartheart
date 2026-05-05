-- Replace daily quota tracking with a lifetime free analyses counter.
-- Each user gets exactly QUOTA_FREE_LIMIT (default 3) free analyses, then requires subscription.

ALTER TABLE users ADD COLUMN IF NOT EXISTS free_analyses_used INT NOT NULL DEFAULT 0;

-- Index for fast lookups during checkQuota and GetQuotaInfo.
CREATE INDEX IF NOT EXISTS idx_users_free_analyses_used ON users(free_analyses_used);

-- Note: user_daily_usage table is kept intact for historical records but is no longer written to.
-- It can be dropped in a future cleanup migration if desired.
