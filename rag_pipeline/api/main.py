"""FastAPI server for the SmartHeart RAG pipeline."""

import asyncio
import contextlib
import hmac
import logging
import os
import threading
import time

import chromadb
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer

from rag_pipeline.config import env_int

logger = logging.getLogger("rag_api")
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)

app = FastAPI(title="SmartHeart RAG API", version="1.0.0")

# Lazy-loaded globals (initialised on startup in a background thread)
_engine = None
_engine_lock = threading.Lock()
_chain = None
_chain_lock = threading.Lock()
_ready = threading.Event()

# LLM config (read once on first _get_chain call)
_llm_model: str = ""
_llm_temperature: float = 0.2


class QueryRequest(BaseModel):
    question: str = Field(..., min_length=2, max_length=2000)
    n_results: int = Field(default=5, ge=1, le=20)


class Source(BaseModel):
    doc_name: str
    chunk_index: int
    score: float
    preview: str


class QueryMeta(BaseModel):
    model: str
    temperature: float
    n_results: int

class QueryResponse(BaseModel):
    answer: str
    sources: list[Source]
    elapsed_ms: int
    meta: QueryMeta | None = None


def _get_engine(force_rebuild: bool = False):
    """Build or return the cached hybrid search engine.

    If ChromaDB already contains an indexed collection, reuses it
    instead of re-processing all documents (saves minutes on startup).
    When force_rebuild is True, always rebuilds from source documents.
    """
    global _engine
    if _engine is not None and not force_rebuild:
        return _engine

    with _engine_lock:
        # Double-check after acquiring lock.
        if _engine is not None and not force_rebuild:
            return _engine

        from rag_pipeline.bm25 import BM25Index
        from rag_pipeline.config import (
            CHROMA_PATH,
            CHUNK_MAX_CHARS,
            CHUNK_MIN_CHARS,
            CHUNK_OVERLAP_CHARS,
            CHUNK_TARGET_CHARS,
            COLLECTION_NAME,
            EMBED_BATCH_SIZE,
            EMBED_MODEL_NAME,
            HNSW_SPACE,
            RRF_K,
            RRF_W_BM25,
            RRF_W_VECTOR,
            SOURCE_DIR,
        )
        from rag_pipeline.hybrid import HybridSearchEngine
        from rag_pipeline.ingestion import (
            chunk_documents,
            discover_files,
            embed_passages,
            load_and_clean,
        )
        from rag_pipeline.tokenization import medical_ru_tokenizer

        logger.info("Initializing RAG engine...")

        embed_model = SentenceTransformer(EMBED_MODEL_NAME)

        # Try to reuse an existing ChromaDB collection.
        client = chromadb.PersistentClient(path=CHROMA_PATH)
        collection = None
        if not force_rebuild:
            try:
                collection = client.get_collection(
                    name=COLLECTION_NAME,
                )
                count = collection.count()
                if count > 0:
                    logger.info(
                        "Reusing existing ChromaDB collection (%d chunks)", count,
                    )
                else:
                    collection = None  # empty — rebuild
            except Exception as exc:
                logger.info("ChromaDB collection not found, will rebuild: %s", exc)
                collection = None

        if collection is None:
            # Full rebuild: load docs → chunk → embed → index.
            files = discover_files(SOURCE_DIR)
            if not files:
                raise RuntimeError(f"No documents found in {SOURCE_DIR.resolve()}")

            raw_documents = load_and_clean(files)
            all_chunk_texts, all_chunk_ids, all_chunk_metas = chunk_documents(
                raw_documents,
                target_chars=CHUNK_TARGET_CHARS,
                min_chars=CHUNK_MIN_CHARS,
                max_chars=CHUNK_MAX_CHARS,
                overlap_chars=CHUNK_OVERLAP_CHARS,
            )

            with contextlib.suppress(Exception):
                client.delete_collection(COLLECTION_NAME)

            collection = client.get_or_create_collection(
                name=COLLECTION_NAME,
                metadata={"hnsw:space": HNSW_SPACE},
            )

            for start in range(0, len(all_chunk_texts), EMBED_BATCH_SIZE):
                end = min(start + EMBED_BATCH_SIZE, len(all_chunk_texts))
                batch_emb = embed_passages(embed_model, all_chunk_texts[start:end])
                collection.add(
                    ids=all_chunk_ids[start:end],
                    documents=all_chunk_texts[start:end],
                    metadatas=all_chunk_metas[start:end],
                    embeddings=batch_emb,
                )
            logger.info("Indexed %d chunks into ChromaDB", len(all_chunk_texts))
        else:
            # Collection exists — still need chunks for BM25.
            all_data = collection.get(include=["documents"])
            all_chunk_ids = all_data["ids"]
            all_chunk_texts = all_data["documents"]

        # Build BM25 index (always in-memory, fast).
        bm25 = BM25Index(tokenizer=medical_ru_tokenizer)
        bm25.build(ids=all_chunk_ids, documents=all_chunk_texts)

        _engine = HybridSearchEngine(
            collection=collection,
            embed_model=embed_model,
            bm25=bm25,
            rrf_k=RRF_K,
            w_vector=RRF_W_VECTOR,
            w_bm25=RRF_W_BM25,
        )
        logger.info("RAG engine ready (%d chunks)", len(all_chunk_ids))
        return _engine


def _get_chain():
    """Build or return the cached LLM chain."""
    global _chain, _llm_model, _llm_temperature
    if _chain is not None:
        return _chain

    with _chain_lock:
        if _chain is not None:
            return _chain

        from rag_pipeline.generation import build_llm, build_prompt

        base_url = os.getenv("LLM_BASE_URL", "https://api.openai.com/v1")
        api_key = os.getenv("LLM_API_KEY", "")
        if not api_key:
            raise RuntimeError("LLM_API_KEY environment variable is required")

        _llm_model = os.getenv("LLM_MODEL", "gpt-4o")
        try:
            _llm_temperature = float(os.getenv("LLM_TEMPERATURE", "0.2"))
        except ValueError as err:
            raw = os.getenv("LLM_TEMPERATURE")
            raise RuntimeError(f"Invalid LLM_TEMPERATURE: {raw!r}") from err

        prompt = build_prompt()
        llm = build_llm(
            base_url=base_url, api_key=api_key,
            model=_llm_model, temperature=_llm_temperature,
        )
        _chain = prompt | llm
        logger.info("LLM chain ready (model=%s, base_url=%s)", _llm_model, base_url)
        return _chain


def _warmup():
    """Pre-load the engine at startup so the first request isn't blocked."""
    try:
        _get_engine()
        _ready.set()
        logger.info("Warmup complete — engine ready")
    except Exception:
        logger.exception("Warmup failed")


_shutting_down = threading.Event()


@app.on_event("startup")
def startup():
    threading.Thread(target=_warmup, daemon=True).start()


@app.on_event("shutdown")
def shutdown():
    _shutting_down.set()
    logger.info("Shutdown signal received")


@app.get("/health")
def health():
    """Liveness probe — is the process alive?"""
    if _shutting_down.is_set():
        raise HTTPException(status_code=503, detail="shutting down")
    return {"status": "ok"}


@app.get("/ready")
def ready():
    """Readiness probe — is the engine loaded and ready to serve?"""
    if not _ready.is_set():
        raise HTTPException(status_code=503, detail="warming up")
    return {"status": "ready"}


def _build_sources(items: list[dict], limit: int = 6) -> list[Source]:
    return [
        Source(
            doc_name=it["meta"].get("doc_name", "unknown"),
            chunk_index=it["meta"].get("chunk_index", 0),
            score=round(it["combined"], 4),
            preview=it["doc"][:200].replace("\n", " ").strip(),
        )
        for it in items[:limit]
    ]


class SearchResponse(BaseModel):
    sources: list[Source]
    elapsed_ms: int


def _search_sync(question: str, n_results: int) -> tuple[list[dict], float]:
    start = time.monotonic()
    engine = _get_engine()
    from rag_pipeline.generation import retrieve_context
    _, items = retrieve_context(engine, question, n_results=n_results)
    elapsed_ms = (time.monotonic() - start) * 1000
    return items, elapsed_ms


@app.post("/search", response_model=SearchResponse)
async def search(req: QueryRequest):
    """Retrieval only — no LLM call. Useful for testing search quality."""
    try:
        items, elapsed_ms = await asyncio.to_thread(
            _search_sync, req.question, req.n_results,
        )
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from None

    return SearchResponse(
        sources=_build_sources(items), elapsed_ms=int(elapsed_ms),
    )


def _query_sync(question: str, n_results: int) -> tuple[str, list[dict], float]:
    """Run retrieval + LLM in a thread (blocking I/O)."""
    start = time.monotonic()
    engine = _get_engine()
    chain = _get_chain()

    from rag_pipeline.generation import get_llm_answer, retrieve_context

    context, items = retrieve_context(engine, question, n_results=n_results)

    last_exc = None
    for attempt in range(3):
        try:
            answer = get_llm_answer(chain, question, context)
            break
        except Exception as exc:
            last_exc = exc
            logger.warning("LLM call attempt %d failed: %s", attempt + 1, exc)
            max_retries = 2
            if attempt < max_retries:
                time.sleep(1.0 * (attempt + 1))
    else:
        raise RuntimeError(f"LLM call failed after 3 attempts: {last_exc}")

    elapsed_ms = (time.monotonic() - start) * 1000
    return answer, items, elapsed_ms


QUERY_TIMEOUT_S = env_int("RAG_QUERY_TIMEOUT", 90)


@app.post("/query", response_model=QueryResponse)
async def query(req: QueryRequest):
    try:
        answer, items, elapsed_ms = await asyncio.wait_for(
            asyncio.to_thread(_query_sync, req.question, req.n_results),
            timeout=QUERY_TIMEOUT_S,
        )
    except asyncio.TimeoutError:
        logger.exception(
            "Query timed out after %ds: %s", QUERY_TIMEOUT_S, req.question[:80],
        )
        raise HTTPException(
            status_code=504, detail="query timed out",
        ) from None
    except RuntimeError as exc:
        detail = str(exc)
        if "LLM call failed" in detail:
            raise HTTPException(
                status_code=502, detail="LLM service error",
            ) from None
        raise HTTPException(status_code=503, detail=detail) from None

    elapsed = int(elapsed_ms)
    logger.info("query answered in %dms: %s", elapsed, req.question[:80])

    return QueryResponse(
        answer=answer,
        sources=_build_sources(items),
        elapsed_ms=elapsed,
        meta=QueryMeta(
            model=_llm_model,
            temperature=_llm_temperature,
            n_results=req.n_results,
        ),
    )


class ReindexResponse(BaseModel):
    status: str
    chunks: int
    elapsed_ms: int


@app.post("/admin/reindex", response_model=ReindexResponse)
def reindex(req: Request):
    """Force-rebuild the search index from source documents."""
    admin_key = os.getenv("ADMIN_API_KEY", "")
    provided = req.headers.get("X-Admin-Key", "")
    if not admin_key or not hmac.compare_digest(provided, admin_key):
        raise HTTPException(status_code=403, detail="forbidden")

    start = time.monotonic()

    try:
        engine = _get_engine(force_rebuild=True)
    except RuntimeError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from None

    chunk_count = engine.collection.count()
    elapsed_ms = int((time.monotonic() - start) * 1000)
    logger.info("reindex completed in %dms: %d chunks", elapsed_ms, chunk_count)

    return ReindexResponse(status="ok", chunks=chunk_count, elapsed_ms=elapsed_ms)
