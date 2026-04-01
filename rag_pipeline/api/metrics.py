"""Prometheus metrics for the RAG pipeline."""

from prometheus_client import Counter, Gauge, Histogram

# --- Query metrics ---

QUERY_TOTAL = Counter(
    "rag_query_total",
    "Total RAG queries",
    ["endpoint"],  # "query" or "search"
)

QUERY_ERRORS = Counter(
    "rag_query_errors_total",
    "Total failed RAG queries",
    ["endpoint", "error_type"],  # "timeout", "llm_error", "engine_error"
)

QUERY_LATENCY = Histogram(
    "rag_query_duration_seconds",
    "RAG query latency",
    ["endpoint"],
    buckets=[0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 90],
)

# --- LLM metrics ---

LLM_CALLS = Counter(
    "rag_llm_calls_total",
    "Total LLM API calls (including retries)",
)

LLM_RETRIES = Counter(
    "rag_llm_retries_total",
    "Total LLM retry attempts",
)

LLM_LATENCY = Histogram(
    "rag_llm_duration_seconds",
    "LLM call latency (single attempt)",
    buckets=[0.5, 1, 2, 5, 10, 20, 30, 60],
)

# --- Index metrics ---

INDEX_CHUNKS = Gauge(
    "rag_index_chunks",
    "Number of chunks in the current index",
)

INDEX_REBUILD_TOTAL = Counter(
    "rag_index_rebuild_total",
    "Total index rebuilds",
)

INDEX_REBUILD_LATENCY = Histogram(
    "rag_index_rebuild_duration_seconds",
    "Index rebuild latency",
    buckets=[1, 5, 10, 30, 60, 120, 300],
)

# --- Engine state ---

ENGINE_READY = Gauge(
    "rag_engine_ready",
    "Whether the RAG engine is ready (1=ready, 0=not ready)",
)
