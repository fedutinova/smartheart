-- Add subscription support to users table.
-- subscription_expires_at: when set and in the future, user has unlimited analyses.
ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_expires_at TIMESTAMPTZ;

-- Add payment_type to payments to distinguish per-analysis from subscription purchases.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payment_type TEXT NOT NULL DEFAULT 'analyses';
-- payment_type values: 'analyses' (existing), 'subscription'
