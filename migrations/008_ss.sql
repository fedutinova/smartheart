-- Backfill: link files from GPT sub-requests to their parent ECG requests.
-- Old ECG handler created file records under the GPT request_id, not the ECG request_id.
-- This migration copies those file records so they also appear under the ECG request.

INSERT INTO files (id, request_id, original_filename, file_type, file_size, s3_bucket, s3_key, s3_url, created_at)
SELECT
    gen_random_uuid(),
    r_ecg.id,
    f.original_filename,
    f.file_type,
    f.file_size,
    f.s3_bucket,
    f.s3_key,
    f.s3_url,
    f.created_at
FROM responses resp
JOIN requests r_ecg ON r_ecg.id = resp.request_id
JOIN files f ON f.request_id = (resp.content::jsonb ->> 'gpt_request_id')::uuid
WHERE resp.content IS NOT NULL
  AND resp.content <> ''
  AND resp.content ~ '^\s*\{.*\}\s*$'
  AND (resp.content::jsonb ->> 'gpt_request_id') IS NOT NULL
  AND (resp.content::jsonb ->> 'gpt_request_id') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM files ef
      WHERE ef.request_id = r_ecg.id
  );
