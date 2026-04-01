# Демонстрационный скрипт, повторяющий шаги ноутбука без изменений логики.
# Теперь документы берутся автоматически из папки ./documents (включая подпапки)
# 1) pip install -r requirements.txt
# 2) Положите ваши PDF/Docx/Doc/TXT/MD/HTML/HTM/RTF в ./documents
# 3) python handover_demo.py

import os

from rag_pipeline.bm25 import BM25Index
from rag_pipeline.config import (
    CHUNK_MAX_CHARS,
    CHUNK_MIN_CHARS,
    CHUNK_OVERLAP_CHARS,
    CHUNK_TARGET_CHARS,
    EMBED_MODEL_NAME,
    SOURCE_DIR,
)
from rag_pipeline.generation import (
    build_llm,
    build_prompt,
    enhanced_query_with_llm,
)
from rag_pipeline.hybrid import HybridSearchEngine
from rag_pipeline.ingestion import (
    build_chroma_index,
    chunk_documents,
    discover_files,
    load_and_clean,
)
from rag_pipeline.tokenization import medical_ru_tokenizer


def main():
    # 0) Источники — собираем автоматически из SOURCE_DIR
    files = discover_files(SOURCE_DIR)
    print(f"Найдено файлов в {SOURCE_DIR.resolve()}: {len(files)}")
    if not files:
        print(
            "Положите документы (PDF/Docx/Doc/TXT/MD/HTML/HTM/RTF) "
            "в папку ./documents и запустите снова."
        )
        return

    # 1) Загрузка и очистка
    raw_documents = load_and_clean(files)
    print(f"Обработано документов: {len(raw_documents)}")
    total_characters = sum(len(d["content"]) for d in raw_documents)
    print(f"Символов всего: {total_characters:,}")

    # 2) Чанкинг
    all_chunk_texts, all_chunk_ids, all_chunk_metas = chunk_documents(
        raw_documents,
        target_chars=CHUNK_TARGET_CHARS,
        min_chars=CHUNK_MIN_CHARS,
        max_chars=CHUNK_MAX_CHARS,
        overlap_chars=CHUNK_OVERLAP_CHARS,
    )
    print(f"Сформировано чанков: {len(all_chunk_texts)}")
    if all_chunk_texts:
        avg_len = sum(len(x) for x in all_chunk_texts) / len(all_chunk_texts)
        print(f"Средняя длина чанка: {avg_len:.1f} символов")
        print(f"Пример чанка (первые 400 символов):\n{all_chunk_texts[0][:400]}...")

    # 3) Индексация в Chroma + эмбеддинги
    collection, embed_model = build_chroma_index(
        texts=all_chunk_texts,
        ids=all_chunk_ids,
        metas=all_chunk_metas,
        embed_model_name=EMBED_MODEL_NAME,
    )

    # 4) BM25
    bm25 = BM25Index(tokenizer=medical_ru_tokenizer)
    bm25.build(ids=all_chunk_ids, documents=all_chunk_texts)
    print("BM25 индекс построен.")

    # 5) Гибридный движок
    engine = HybridSearchEngine(
        collection=collection,
        embed_model=embed_model,
        bm25=bm25,
        rrf_k=60.0,
        w_vector=1.0,
        w_bm25=1.0,
    )

    # 6) Пример гибридного поиска
    query = "диагностика фибрилляции предсердий ЭКГ критерии"
    res = engine.search(query, n_results=5, vector_k=40, bm25_k=40)
    print("\n==== TOP RESULTS ====")
    for rank, (cid, score, doc, meta) in enumerate(
        zip(res.ids, res.combined_scores, res.documents, res.metadatas, strict=True),
        start=1,
    ):
        v = res.vector_scores.get(cid)
        b = res.bm25_scores.get(cid)
        v_str = f"{v:.4f}" if v is not None else "n/a"
        b_str = f"{b:.4f}" if b is not None else "n/a"
        print(f"\n#{rank} id={cid}")
        print(f"combined={score:.6f} | vector={v_str} | bm25={b_str}")
        print(
            f"source={meta.get('doc_name')} | "
            f"chunk_index={meta.get('chunk_index')} | "
            f"chunk_len={meta.get('chunk_len')}"
        )
        snippet = doc[:700].replace("\n", " ")
        print(f"text: {snippet}...")

    # 7) Генерация ответа (LLM)
    base_url = os.getenv("LLM_BASE_URL", "https://bothub.chat/api/v2/openai/v1")
    api_key = os.getenv("LLM_API_KEY", "")
    if not api_key:
        print("Установите LLM_API_KEY для генерации ответа.")
        return
    prompt = build_prompt()
    llm = build_llm(base_url=base_url, api_key=api_key, temperature=0.2)
    chain = prompt | llm

    enhanced_response = enhanced_query_with_llm(
        engine, chain,
        "Признаки предсердной тахикардии?",
        n_results=5, vector_k=40, bm25_k=40,
    )
    print("\n==== LLM ANSWER ====\n")
    print(enhanced_response)


if __name__ == "__main__":
    main()
