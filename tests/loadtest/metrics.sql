-- H2 hypothesis metrics calculation.
-- Pass $1 = start timestamp of the test run (e.g., '2026-04-03 12:00:00+00').
--
-- Usage:
--   psql "$DATABASE_URL" -v since="'2026-04-03 12:00:00+00'" -f metrics.sql

WITH raw AS (
    SELECT
        r.id                                                    AS request_id,
        CASE resp.model
            WHEN 'ekg_structured_v1' THEN 'ecg'
            WHEN 'rag_query'         THEN 'kb'
            ELSE resp.model
        END                                                     AS scenario,
        r.created_at,
        r.updated_at                                            AS result_ready_at,
        -- Use the greater of wall-clock delta and processing_time_ms.
        -- In sync mode created_at == updated_at, so wall-clock is ~0.
        GREATEST(
            EXTRACT(EPOCH FROM (r.updated_at - r.created_at)),
            COALESCE(resp.processing_time_ms, 0) / 1000.0
        )                                                       AS duration_sec,
        r.status
    FROM requests r
    JOIN responses resp ON resp.request_id = r.id
    WHERE resp.model IN ('ekg_structured_v1', 'rag_query')
      AND r.created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
),
metrics AS (
    SELECT
        scenario,
        COUNT(*)                                                         AS n_all,
        COUNT(*) FILTER (WHERE status = 'completed')                     AS n_success,
        COUNT(*) FILTER (WHERE status = 'completed' AND duration_sec <= 30) AS n_under_30s,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_sec)
            FILTER (WHERE status = 'completed')                          AS p95_sec
    FROM raw
    GROUP BY scenario
)
SELECT
    scenario,
    n_all,
    n_success,
    n_under_30s,
    ROUND(n_success::numeric    / NULLIF(n_all, 0),     4) AS success_rate,
    ROUND(n_under_30s::numeric / NULLIF(n_success, 0), 4) AS share_under_30s,
    ROUND(p95_sec::numeric, 2)                              AS p95_time_sec
FROM metrics
ORDER BY scenario;
