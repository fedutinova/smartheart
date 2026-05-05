-- Performance optimization: add composite indexes and missing indexes

-- For GetUserRequests with pagination - frequently used on list page
CREATE INDEX IF NOT EXISTS idx_requests_user_created ON requests(user_id, created_at DESC);

-- For GetRequestsByStatus queries - filter by status, order by created_at
CREATE INDEX IF NOT EXISTS idx_requests_status_created ON requests(status, created_at DESC);

-- For EKG chat queries - find messages by request, order by timestamp
CREATE INDEX IF NOT EXISTS idx_ecg_chat_request_created ON ecg_chat_messages(request_id, created_at);

-- For user quota checks - find daily usage for today
CREATE INDEX IF NOT EXISTS idx_user_daily_usage_user_date ON user_daily_usage(user_id, usage_date DESC);

-- For response lookups by created_at (recent responses)
CREATE INDEX IF NOT EXISTS idx_responses_created_at ON responses(created_at DESC);

-- For KB cache searches - find entries by creation date
CREATE INDEX IF NOT EXISTS idx_kb_cache_created_on ON kb_cache(created_at DESC);

-- For payments status filtering - show pending/completed separately
CREATE INDEX IF NOT EXISTS idx_payments_user_status ON payments(user_id, status);

-- For ECG analysis - filter by status quickly
CREATE INDEX IF NOT EXISTS idx_ecg_chat_messages_user_request ON ecg_chat_messages(user_id, request_id);

-- Index for refresh token lookups by user and expiry
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_active
  ON refresh_tokens(user_id, expires_at DESC);

-- Index for password reset token lookups
CREATE INDEX IF NOT EXISTS idx_password_reset_active
  ON password_reset_tokens(token_hash, expires_at)
  WHERE used_at IS NULL;

-- For RAG feedback queries - find recent feedback efficiently
CREATE INDEX IF NOT EXISTS idx_rag_feedback_user_created ON rag_feedback(user_id, created_at DESC);

-- Analysis: EXPLAIN ANALYZE these queries to verify improvement
-- SELECT * FROM requests WHERE user_id = ? ORDER BY created_at DESC LIMIT 50 OFFSET 0;
-- SELECT * FROM requests WHERE status = 'completed' ORDER BY created_at DESC;
-- SELECT * FROM ecg_chat_messages WHERE request_id = ? ORDER BY created_at;
