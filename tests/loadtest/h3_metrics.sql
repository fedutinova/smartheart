-- H3 hypothesis metrics: hybrid KB cache effectiveness.
-- Usage: psql "$DATABASE_URL" -v since="'2026-04-04 06:00:00+00'" -f h3_metrics.sql

WITH raw AS (
    SELECT
        r.id AS request_id,
        resp.processing_time_ms,
        resp.cache_status,
        resp.cache_match_method,
        resp.cache_trigram_similarity,
        resp.cache_vector_similarity,
        resp.cache_combined_similarity,
        r.status
    FROM requests r
    JOIN responses resp ON resp.request_id = r.id
    WHERE resp.model = 'rag_query'
      AND r.created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
),
summary AS (
    SELECT
        COUNT(*)                                      AS n_total,
        COUNT(*) FILTER (WHERE cache_status = 'HIT') AS n_cache_hits,
        COUNT(*) FILTER (WHERE cache_status = 'MISS') AS n_rag_calls,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY processing_time_ms)
            FILTER (WHERE cache_status = 'HIT')      AS p95_cached_ms,
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY processing_time_ms)
            FILTER (WHERE cache_status = 'MISS')     AS p95_uncached_ms,
        AVG(processing_time_ms) FILTER (WHERE cache_status = 'HIT') AS avg_cached_ms,
        AVG(processing_time_ms) FILTER (WHERE cache_status = 'MISS') AS avg_uncached_ms,
        AVG(cache_trigram_similarity) FILTER (WHERE cache_status = 'HIT') AS avg_trigram_similarity,
        AVG(cache_vector_similarity) FILTER (WHERE cache_status = 'HIT') AS avg_vector_similarity,
        AVG(cache_combined_similarity) FILTER (WHERE cache_status = 'HIT') AS avg_combined_similarity
    FROM raw
    WHERE status = 'completed'
)
SELECT
    n_total,
    n_cache_hits,
    n_rag_calls,
    ROUND(n_cache_hits::numeric / NULLIF(n_total, 0), 4) AS cache_hit_rate,
    ROUND(p95_cached_ms::numeric, 0) AS p95_cached_ms,
    ROUND(p95_uncached_ms::numeric, 0) AS p95_uncached_ms,
    ROUND(avg_cached_ms::numeric, 0) AS avg_cached_ms,
    ROUND(avg_uncached_ms::numeric, 0) AS avg_uncached_ms,
    ROUND(avg_trigram_similarity::numeric, 4) AS avg_trigram_similarity,
    ROUND(avg_vector_similarity::numeric, 4) AS avg_vector_similarity,
    ROUND(avg_combined_similarity::numeric, 4) AS avg_combined_similarity
FROM summary;

WITH raw AS (
    SELECT
        resp.processing_time_ms,
        resp.cache_status,
        resp.cache_match_method,
        r.status
    FROM requests r
    JOIN responses resp ON resp.request_id = r.id
    WHERE resp.model = 'rag_query'
      AND r.created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
)
SELECT
    cache_status,
    cache_match_method,
    COUNT(*) AS n,
    ROUND(AVG(processing_time_ms)::numeric, 0) AS avg_ms,
    ROUND(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY processing_time_ms)::numeric, 0) AS p95_ms
FROM raw
WHERE status = 'completed'
GROUP BY cache_status, cache_match_method
ORDER BY cache_status, cache_match_method;

-- Cache contents observed during the run.
SELECT
    'Cache contents' AS label,
    question_normalized,
    hit_count,
    ROUND(EXTRACT(EPOCH FROM (expires_at - created_at)) / 3600) AS ttl_hours
FROM kb_cache
WHERE created_at >= COALESCE(:since::timestamptz, NOW() - INTERVAL '1 hour')
ORDER BY hit_count DESC
LIMIT 30;
