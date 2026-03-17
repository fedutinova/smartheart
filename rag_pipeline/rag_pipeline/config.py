from pathlib import Path

# Папка с исходными документами внутри проекта
SOURCE_DIR = Path("documents")
SOURCE_DIR.mkdir(parents=True, exist_ok=True)

# Папка для сохранения очищенных текстов (опционально)
PROCESSED_DIR = Path("processed_docs")
PROCESSED_DIR.mkdir(parents=True, exist_ok=True)

# Параметры чанкинга (как в ноутбуке)
CHUNK_TARGET_CHARS = 1900
CHUNK_MIN_CHARS = 1400
CHUNK_MAX_CHARS = 2600
CHUNK_OVERLAP_CHARS = 250

# Chroma
CHROMA_PATH = "./chroma_db_4"
COLLECTION_NAME = "cardio_docs_hybrid"
HNSW_SPACE = "cosine"

# Embeddings
EMBED_MODEL_NAME = "intfloat/multilingual-e5-base"
