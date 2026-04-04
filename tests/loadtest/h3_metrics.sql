-- H3 hypothesis metrics: semantic cache effectiveness.
-- Usage: psql "$DATABASE_URL" -v since="'2026-04-04 06:00:00+00'" -f h3_metrics.sql

WITH raw AS (
    SELECT
        r.id AS request_id,
        resp.processing_time_ms,
        CASE
            WHEN resp.processing_time_ms < 1000 THEN 'cache'
            ELSE 'rag'
        END AS source,
        r.status
    FROM requests r
    JOIN responses resp ON resp.request_id = r.id
    WHERE resp.model = 'rag_query'
      AND r.created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
),
summary AS (
    SELECT
        COUNT(*)                                         AS n_total,
        COUNT(*) FILTER (WHERE source = 'cache')         AS n_cache_hits,
        COUNT(*) FILTER (WHERE source = 'rag')           AS n_rag_calls,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY processing_time_ms)
            FILTER (WHERE source = 'cache')              AS p95_cached_ms,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY processing_time_ms)
            FILTER (WHERE source = 'rag')                AS p95_uncached_ms,
        AVG(processing_time_ms) FILTER (WHERE source = 'cache') AS avg_cached_ms,
        AVG(processing_time_ms) FILTER (WHERE source = 'rag')   AS avg_uncached_ms
    FROM raw
    WHERE status = 'completed'
)
SELECT
    n_total,
    n_cache_hits,
    n_rag_calls,
    ROUND(n_cache_hits::numeric / NULLIF(n_total, 0), 4) AS cache_hit_rate,
    ROUND(p95_cached_ms::numeric, 0)   AS p95_cached_ms,
    ROUND(p95_uncached_ms::numeric, 0) AS p95_uncached_ms,
    ROUND(avg_cached_ms::numeric, 0)   AS avg_cached_ms,
    ROUND(avg_uncached_ms::numeric, 0) AS avg_uncached_ms
FROM summary;

-- Per-group breakdown (which topics get cache hits).
SELECT
    'Cache contents' AS label,
    question_normalized,
    hit_count,
    ROUND(EXTRACT(EPOCH FROM (expires_at - created_at)) / 3600) AS ttl_hours
FROM kb_cache
WHERE created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
ORDER BY hit_count DESC
LIMIT 30;
