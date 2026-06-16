"""Retrieval-only probe: ingest docs, run the LVH query, show what lands in context.

No LLM call — we only inspect what the hybrid retriever feeds the prompt as {context}.
"""
import re

from rag_pipeline.bm25 import BM25Index
from rag_pipeline.config import (
    CHUNK_MAX_CHARS,
    CHUNK_MIN_CHARS,
    CHUNK_OVERLAP_CHARS,
    CHUNK_TARGET_CHARS,
    EMBED_MODEL_NAME,
    SOURCE_DIR,
)
from rag_pipeline.hybrid import HybridSearchEngine
from rag_pipeline.ingestion import (
    build_chroma_index,
    chunk_documents,
    discover_files,
    load_and_clean,
)
from rag_pipeline.tokenization import medical_ru_tokenizer

LEFT = re.compile(r"лев\w*\s+желудоч\w*|глж", re.IGNORECASE)
RIGHT = re.compile(r"прав\w*\s+желудоч\w*|гпж", re.IGNORECASE)


def tag(text: str) -> str:
    l = len(LEFT.findall(text))
    r = len(RIGHT.findall(text))
    return f"left_hits={l} right_hits={r}"


def main():
    files = discover_files(SOURCE_DIR)
    print(f"files: {len(files)}")
    raw = load_and_clean(files)
    texts, ids, metas = chunk_documents(
        raw,
        target_chars=CHUNK_TARGET_CHARS,
        min_chars=CHUNK_MIN_CHARS,
        max_chars=CHUNK_MAX_CHARS,
        overlap_chars=CHUNK_OVERLAP_CHARS,
    )
    print(f"chunks: {len(texts)}")

    collection, embed_model = build_chroma_index(
        texts=texts, ids=ids, metas=metas, embed_model_name=EMBED_MODEL_NAME,
    )
    bm25 = BM25Index(tokenizer=medical_ru_tokenizer)
    bm25.build(ids=ids, documents=texts)

    engine = HybridSearchEngine(
        collection=collection, embed_model=embed_model, bm25=bm25,
        rrf_k=60.0, w_vector=1.0, w_bm25=1.0,
    )

    query = "критерии гипертрофии левого желудочка на ЭКГ"
    print(f"\n==== QUERY: {query} ====")
    res = engine.search(query, n_results=5, vector_k=40, bm25_k=40)
    for rank, (cid, score, doc, meta) in enumerate(
        zip(res.ids, res.combined_scores, res.documents, res.metadatas, strict=True),
        start=1,
    ):
        v = res.vector_scores.get(cid)
        b = res.bm25_scores.get(cid)
        v_str = f"{v:.4f}" if v is not None else "n/a"
        b_str = f"{b:.4f}" if b is not None else "n/a"
        snippet = doc[:300].replace("\n", " ")
        print(f"\n#{rank} combined={score:.6f} | vector={v_str} | bm25={b_str}")
        print(f"  source={meta.get('doc_name')} chunk={meta.get('chunk_index')} | {tag(doc)}")
        print(f"  text: {snippet}...")


if __name__ == "__main__":
    main()
