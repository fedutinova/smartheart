import contextlib
import hashlib
import logging
from pathlib import Path
from typing import Any

import chromadb
from markitdown import MarkItDown
from sentence_transformers import SentenceTransformer

from .chunking import chunk_units_semantic, split_to_units
from .config import (
    CHROMA_PATH,
    COLLECTION_NAME,
    EMBED_BATCH_SIZE,
    HNSW_SPACE,
    PROCESSED_DIR,
)
from .text_clean import clean_text

logger = logging.getLogger(__name__)

ALLOWED_EXTS = {".pdf", ".docx", ".doc", ".txt", ".md", ".html", ".htm", ".rtf"}

def discover_files(root: Path) -> list[Path]:
    """Рекурсивно обходит папку и возвращает файлы с поддерживаемыми расширениями."""
    files: list[Path] = []
    if not root.exists():
        logger.warning("Папка с документами не найдена: %s", root.resolve())
        return files
    for p in root.rglob("*"):
        if p.is_file() and p.suffix.lower() in ALLOWED_EXTS:
            files.append(p)
    return sorted(files)

def stable_doc_id(s: str) -> str:
    return hashlib.sha1(s.encode("utf-8")).hexdigest()[:12]

def load_and_clean(files: list[Path]) -> list[dict[str, Any]]:
    """Загружает файлы через MarkItDown, чистит текст и возвращает список словарей."""
    md = MarkItDown()
    out: list[dict[str, Any]] = []
    for file_path in files:
        if not file_path.exists():
            logger.warning("Файл не найден: %s", file_path)
            continue
        logger.info("Обработка: %s", file_path.name)
        result = md.convert(file_path)
        text = (result.text_content or "")
        text = clean_text(text)

        out.append({
            "source": str(file_path),
            "doc_name": file_path.name,
            "content": text,
        })

        # Сохранить очищенный документ для аудита.
        did = stable_doc_id(str(file_path))
        out_txt = PROCESSED_DIR / f"{did}_{file_path.stem}.txt"
        try:
            out_txt.write_text(text, encoding="utf-8")
        except OSError as exc:
            logger.warning("Не удалось сохранить %s: %s", out_txt, exc)
    return out

def chunk_documents(
    raw_documents: list[dict[str, Any]],
    target_chars: int,
    min_chars: int,
    max_chars: int,
    overlap_chars: int,
) -> tuple[list[str], list[str], list[dict[str, Any]]]:
    """Делит документы на чанки. Возвращает параллельные списки: texts, ids, metas."""
    all_chunk_texts: list[str] = []
    all_chunk_ids: list[str] = []
    all_chunk_metas: list[dict[str, Any]] = []

    for doc in raw_documents:
        source = doc["source"]
        doc_name = doc["doc_name"]
        content = doc["content"]

        units = split_to_units(content)
        chunks = chunk_units_semantic(
            units,
            target_chars=target_chars,
            min_chars=min_chars,
            max_chars=max_chars,
            overlap_chars=overlap_chars,
        )

        did = stable_doc_id(source)
        for i, ch in enumerate(chunks):
            chunk_id = f"{did}_{i:05d}"
            meta = {
                "source": source,
                "doc_name": doc_name,
                "doc_id": did,
                "chunk_index": i,
                "chunk_len": len(ch),
            }
            all_chunk_ids.append(chunk_id)
            all_chunk_texts.append(ch)
            all_chunk_metas.append(meta)
    return all_chunk_texts, all_chunk_ids, all_chunk_metas

def embed_passages(
    embed_model: SentenceTransformer, texts: list[str],
) -> list[list[float]]:
    pref = [f"passage: {t}" for t in texts]
    embs = embed_model.encode(
        pref, normalize_embeddings=True, batch_size=EMBED_BATCH_SIZE,
    )
    return [e.tolist() for e in embs]

def build_chroma_index(
    texts: list[str],
    ids: list[str],
    metas: list[dict[str, Any]],
    embed_model_name: str,
):
    """Создаёт/пересоздаёт Chroma коллекцию и записывает туда чанки с эмбеддингами."""
    client = chromadb.PersistentClient(path=CHROMA_PATH)
    with contextlib.suppress(Exception):
        client.delete_collection(COLLECTION_NAME)

    collection = client.get_or_create_collection(
        name=COLLECTION_NAME,
        metadata={"hnsw:space": HNSW_SPACE},
    )
    logger.info("Created collection: %s", collection.name)

    embed_model = SentenceTransformer(embed_model_name)

    for start in range(0, len(texts), EMBED_BATCH_SIZE):
        end = min(start + EMBED_BATCH_SIZE, len(texts))
        batch_emb = embed_passages(embed_model, texts[start:end])

        collection.add(
            ids=ids[start:end],
            documents=texts[start:end],
            metadatas=metas[start:end],
            embeddings=batch_emb,
        )
        logger.info("Добавлено в Chroma: %d/%d", end, len(texts))

    logger.info("Индексация завершена.")
    return collection, embed_model
