-- Create requests table
CREATE TABLE IF NOT EXISTS requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255),
    text_query TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    original_filename VARCHAR(500) NOT NULL,
    file_type VARCHAR(100),
    file_size BIGINT,
    s3_bucket VARCHAR(255),
    s3_key VARCHAR(500) NOT NULL,
    s3_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    model VARCHAR(100),
    tokens_used INTEGER,
    processing_time_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_requests_user_id ON requests(user_id);
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);
CREATE INDEX IF NOT EXISTS idx_files_request_id ON files(request_id);
CREATE INDEX IF NOT EXISTS idx_responses_request_id ON responses(request_id);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_requests_updated_at BEFORE UPDATE ON requests 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
