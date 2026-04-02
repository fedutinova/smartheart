-- Ensure one feedback row per (user, question, answer) for idempotent upserts.
CREATE UNIQUE INDEX IF NOT EXISTS uq_rag_feedback_user_question_answer
    ON rag_feedback (user_id, md5(question), md5(answer));
