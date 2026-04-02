-- Ensure one feedback row per (user, question, answer) for idempotent upserts.
CREATE UNIQUE INDEX IF NOT EXISTS uq_rag_feedback_user_question_answer
    ON rag_feedback (user_id, md5(question), md5(answer));

-- Prevent duplicate pending subscription payments per user.
-- Only one pending subscription payment can exist at a time.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_pending_subscription
    ON payments (user_id, payment_type) WHERE status = 'pending' AND payment_type = 'subscription';
