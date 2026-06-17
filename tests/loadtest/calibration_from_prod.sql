-- Calibration queries for honest load testing.
-- Source of truth = real prod telemetry (no GPT calls spent).
-- Run on prod:  docker exec -i smartheart_postgres psql -U smartheart -d smartheart -f /tests/calibration_from_prod.sql
-- Feed the results into GPT_MOCK_* env vars (see docs/loadtest-plan.md).

\echo '== ECG pipeline GPT wall-time (measurement + interpretation, summed) =='
-- responses.processing_time_ms for structured ECG = totalProcessingTimeMs from
-- the worker = measurement call + interpretation call. This is the per-job GPT
-- wall time a worker is busy, which drives queue saturation.
SELECT
  count(*)                                                              AS n,
  round(avg(processing_time_ms))                                       AS avg_ms,
  round(percentile_cont(0.5)  WITHIN GROUP (ORDER BY processing_time_ms)) AS p50_ms,
  round(percentile_cont(0.9)  WITHIN GROUP (ORDER BY processing_time_ms)) AS p90_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY processing_time_ms)) AS p95_ms,
  round(percentile_cont(0.5)  WITHIN GROUP (ORDER BY tokens_used))        AS p50_tokens
FROM responses
WHERE model = 'ekg_structured_v1'
  AND processing_time_ms IS NOT NULL AND processing_time_ms > 0;

\echo '== RAG answers by cache status (MISS = real LLM path, HIT = cache) =='
-- RAG responses carry cache_status (migration 017); ECG responses do not, so
-- cache_status IS NOT NULL selects the RAG path without hard-coding a model name.
SELECT
  cache_status,
  count(*)                                                              AS n,
  round(avg(processing_time_ms))                                       AS avg_ms,
  round(percentile_cont(0.5)  WITHIN GROUP (ORDER BY processing_time_ms)) AS p50_ms,
  round(percentile_cont(0.95) WITHIN GROUP (ORDER BY processing_time_ms)) AS p95_ms
FROM responses
WHERE cache_status IS NOT NULL
  AND processing_time_ms IS NOT NULL AND processing_time_ms > 0
GROUP BY cache_status
ORDER BY cache_status;
