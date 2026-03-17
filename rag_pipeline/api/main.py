"""FastAPI server for the SmartHeart RAG pipeline."""

import logging
import os
import time

import chromadb
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from sentence_transformers import SentenceTransformer

logger = logging.getLogger("rag_api")
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")

app = FastAPI(title="SmartHeart RAG API", version="1.0.0")

# Lazy-loaded globals (initialised on first request or startup)
_engine = None
_chain = None

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


def _get_engine():
    """Build or return the cached hybrid search engine.

    If ChromaDB already contains an indexed collection, reuses it
    instead of re-processing all documents (saves minutes on startup).
    """
    global _engine
    if _engine is not None:
        return _engine

    from rag_pipeline.config import (
        SOURCE_DIR, CHUNK_TARGET_CHARS, CHUNK_MIN_CHARS,
        CHUNK_MAX_CHARS, CHUNK_OVERLAP_CHARS, EMBED_MODEL_NAME,
        CHROMA_PATH, COLLECTION_NAME, HNSW_SPACE,
    )
    from rag_pipeline.ingestion import discover_files, load_and_clean, chunk_documents, embed_passages
    from rag_pipeline.tokenization import medical_ru_tokenizer
    from rag_pipeline.bm25 import BM25Index
    from rag_pipeline.hybrid import HybridSearchEngine

    logger.info("Initializing RAG engine...")

    embed_model = SentenceTransformer(EMBED_MODEL_NAME)

    # Try to reuse an existing ChromaDB collection.
    client = chromadb.PersistentClient(path=CHROMA_PATH)
    collection = None
    try:
        collection = client.get_collection(
            name=COLLECTION_NAME,
        )
        count = collection.count()
        if count > 0:
            logger.info("Reusing existing ChromaDB collection (%d chunks)", count)
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

        try:
            client.delete_collection(COLLECTION_NAME)
        except Exception:
            pass

        collection = client.get_or_create_collection(
            name=COLLECTION_NAME,
            metadata={"hnsw:space": HNSW_SPACE},
        )

        BATCH = 64
        for start in range(0, len(all_chunk_texts), BATCH):
            end = min(start + BATCH, len(all_chunk_texts))
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
    )
    logger.info("RAG engine ready (%d chunks)", len(all_chunk_ids))
    return _engine


def _get_chain():
    """Build or return the cached LLM chain."""
    global _chain, _llm_model, _llm_temperature
    if _chain is not None:
        return _chain

    from rag_pipeline.generation import build_prompt, build_llm

    base_url = os.getenv("LLM_BASE_URL", "https://api.openai.com/v1")
    api_key = os.getenv("LLM_API_KEY", "")
    if not api_key:
        raise RuntimeError("LLM_API_KEY environment variable is required")

    _llm_model = os.getenv("LLM_MODEL", "gpt-5")
    _llm_temperature = float(os.getenv("LLM_TEMPERATURE", "0.2"))

    prompt = build_prompt()
    llm = build_llm(base_url=base_url, api_key=api_key, model=_llm_model, temperature=_llm_temperature)
    _chain = prompt | llm
    logger.info("LLM chain ready (model=%s, base_url=%s)", _llm_model, base_url)
    return _chain


@app.get("/health")
def health():
    return {"status": "ok"}


class SearchResponse(BaseModel):
    sources: list[Source]
    elapsed_ms: int


@app.post("/search", response_model=SearchResponse)
def search(req: QueryRequest):
    """Retrieval only — no LLM call. Useful for testing search quality."""
    start = time.monotonic()

    try:
        engine = _get_engine()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc))

    from rag_pipeline.generation import retrieve_context

    _, items = retrieve_context(engine, req.question, n_results=req.n_results)

    sources = []
    for it in items[:6]:
        meta = it["meta"]
        sources.append(Source(
            doc_name=meta.get("doc_name", "unknown"),
            chunk_index=meta.get("chunk_index", 0),
            score=round(it["combined"], 4),
            preview=it["doc"][:200].replace("\n", " ").strip(),
        ))

    elapsed_ms = int((time.monotonic() - start) * 1000)
    return SearchResponse(sources=sources, elapsed_ms=elapsed_ms)


@app.post("/query", response_model=QueryResponse)
def query(req: QueryRequest):
    start = time.monotonic()

    try:
        engine = _get_engine()
        chain = _get_chain()
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc))

    from rag_pipeline.generation import retrieve_context, get_llm_answer

    context, items = retrieve_context(engine, req.question, n_results=req.n_results)

    try:
        answer = get_llm_answer(chain, req.question, context)
    except Exception as exc:
        logger.error("LLM call failed: %s", exc)
        raise HTTPException(status_code=502, detail="LLM service error")

    sources = []
    for it in items[:6]:
        meta = it["meta"]
        sources.append(Source(
            doc_name=meta.get("doc_name", "unknown"),
            chunk_index=meta.get("chunk_index", 0),
            score=round(it["combined"], 4),
            preview=it["doc"][:200].replace("\n", " ").strip(),
        ))

    elapsed_ms = int((time.monotonic() - start) * 1000)
    logger.info("query answered in %dms: %s", elapsed_ms, req.question[:80])

    return QueryResponse(
        answer=answer,
        sources=sources,
        elapsed_ms=elapsed_ms,
        meta=QueryMeta(model=_llm_model, temperature=_llm_temperature, n_results=req.n_results),
    )
