import os
from pathlib import Path


def env_int(key: str, default: int) -> int:
    v = os.getenv(key)
    if not v:
        return default
    try:
        return int(v)
    except ValueError as err:
        raise ValueError(f"Invalid integer for {key}: {v!r}") from err


def env_float(key: str, default: float) -> float:
    v = os.getenv(key)
    if not v:
        return default
    try:
        return float(v)
    except ValueError as err:
        raise ValueError(f"Invalid float for {key}: {v!r}") from err


# Папка с исходными документами
SOURCE_DIR = Path(os.getenv("RAG_SOURCE_DIR", "documents"))
SOURCE_DIR.mkdir(parents=True, exist_ok=True)

# Папка для сохранения очищенных текстов (аудит)
PROCESSED_DIR = Path(os.getenv("RAG_PROCESSED_DIR", "processed_docs"))
PROCESSED_DIR.mkdir(parents=True, exist_ok=True)

# Параметры чанкинга
CHUNK_TARGET_CHARS = env_int("RAG_CHUNK_TARGET", 1900)
CHUNK_MIN_CHARS = env_int("RAG_CHUNK_MIN", 1400)
CHUNK_MAX_CHARS = env_int("RAG_CHUNK_MAX", 2600)
CHUNK_OVERLAP_CHARS = env_int("RAG_CHUNK_OVERLAP", 250)

# ChromaDB
CHROMA_PATH = os.getenv("RAG_CHROMA_PATH", "./chroma_db_4")
COLLECTION_NAME = os.getenv("RAG_COLLECTION_NAME", "cardio_docs_hybrid")
HNSW_SPACE = os.getenv("RAG_HNSW_SPACE", "cosine")

# Embeddings
EMBED_MODEL_NAME = os.getenv("RAG_EMBED_MODEL", "intfloat/multilingual-e5-base")
EMBED_BATCH_SIZE = env_int("RAG_EMBED_BATCH_SIZE", 64)

# RRF fusion
RRF_K = env_float("RAG_RRF_K", 60.0)
RRF_W_VECTOR = env_float("RAG_RRF_W_VECTOR", 1.0)
RRF_W_BM25 = env_float("RAG_RRF_W_BM25", 1.0)

# LLM
LLM_MAX_TOKENS = env_int("LLM_MAX_TOKENS", 1000)
