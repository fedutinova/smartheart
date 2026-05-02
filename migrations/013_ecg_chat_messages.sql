-- Contextual chat messages anchored to a specific ECG analysis request.
-- Lets users discuss their ECG result with the assistant; the structured
-- result is loaded from `requests` and injected into the prompt at runtime.

CREATE TABLE IF NOT EXISTS ecg_chat_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id  UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    role        VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant')),
    content     TEXT NOT NULL,
    citations   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ecg_chat_request ON ecg_chat_messages (request_id, created_at);
CREATE INDEX idx_ecg_chat_user ON ecg_chat_messages (user_id);
