-- Export per-request raw data for H2 study.
-- Variant must be supplied externally (async or sync).
-- Usage:
--   psql "$DATABASE_URL" -v since="'2026-04-03 12:00:00+00'" -f export_raw.sql

SELECT
    r.id                                                   AS request_id,
    CASE resp.model
        WHEN 'ekg_structured_v1' THEN 'ecg'
        WHEN 'rag_query'         THEN 'kb'
        ELSE resp.model
    END                                                    AS scenario,
    r.created_at,
    r.updated_at                                           AS result_ready_at,
    ROUND(EXTRACT(EPOCH FROM (r.updated_at - r.created_at))::numeric, 2) AS duration_sec,
    r.status
FROM requests r
JOIN responses resp ON resp.request_id = r.id
WHERE resp.model IN ('ekg_structured_v1', 'rag_query')
  AND r.created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
ORDER BY r.created_at;
