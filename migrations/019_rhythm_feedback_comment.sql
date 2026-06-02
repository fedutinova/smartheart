-- Optional user comment alongside the rhythm-feedback rating. Typically
-- collected when the rating is "inaccurate" to capture what was wrong — a
-- direct retraining signal for the CV/LLM stack. Plain TEXT is fine; the
-- handler caps the length, no need for a server-side limit here.
ALTER TABLE rhythm_feedback ADD COLUMN IF NOT EXISTS comment TEXT;
