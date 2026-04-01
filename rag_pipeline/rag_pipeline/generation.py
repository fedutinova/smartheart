from langchain_core.prompts import PromptTemplate
from langchain_openai import ChatOpenAI

from .config import LLM_MAX_TOKENS
from .hybrid import HybridSearchEngine


def build_prompt() -> PromptTemplate:
    return PromptTemplate(
        input_variables=["context", "question"],
        template="""Ты — врач функциональной диагностики. Отвечай на русском, профессионально, кратко.

Правила:
- Опирайся на документы ниже. Если данных недостаточно — скажи прямо.
- Не назначай лечение/препараты/дозировки.
- Неспецифичный признак ≠ диагноз. Дифференциальный ряд — от вероятного к редкому.
- Каждый вариант привязывай к конкретным ЭКГ-критериям.

Определи тип вопроса и используй соответствующую структуру:

Тип А — пользователь называет диагноз/явление и хочет узнать его признаки.
Примеры: «Фибрилляция предсердий», «Признаки инфаркта миокарда», «АВ-блокада II степени»
Структура: ### Определение → ### ЭКГ-признаки → ### Дифдиагностика → ### Вывод

Тип Б — пользователь описывает ЭКГ-находку и хочет узнать, что это может быть.
Примеры: «Нет зубца P», «Широкий QRS», «Элевация ST в V1-V4», «Удлинённый PQ»
Структура: ### Специфичность → ### Вероятные варианты → ### Подтверждающие критерии → ### Итог

Формат: Markdown, ### заголовки, **жирные** термины, списки. 200–400 слов, без воды.

# ДОКУМЕНТЫ
{context}

# ВОПРОС
{question}
"""
    )

def build_llm(
    base_url: str,
    api_key: str,
    model: str = "gpt-4o",
    temperature: float = 0.2,
    max_tokens: int = LLM_MAX_TOKENS,
    timeout: int = 60,
):
    return ChatOpenAI(
        base_url=base_url, api_key=api_key, model=model,
        temperature=temperature, max_tokens=max_tokens,
        request_timeout=timeout,
    )


def retrieve_context(
    engine: HybridSearchEngine,
    question: str,
    n_results: int = 5,
    vector_k: int = 40,
    bm25_k: int = 40,
):
    res = engine.search(
        question, n_results=n_results,
        vector_k=vector_k, bm25_k=bm25_k,
    )
    items = []
    for cid, score, doc, meta in zip(
        res.ids, res.combined_scores,
        res.documents, res.metadatas, strict=True,
    ):
        vec = res.vector_scores.get(cid)
        bm = res.bm25_scores.get(cid)
        items.append({
            "id": cid,
            "combined": float(score),
            "vector": float(vec) if vec is not None else None,
            "bm25": float(bm) if bm is not None else None,
            "doc": doc,
            "meta": meta or {},
        })
    parts = []
    for it in items:
        doc_name = it["meta"].get("doc_name", "unknown_doc")
        chunk_idx = it["meta"].get("chunk_index", "unknown_chunk")
        parts.append(
            f"[{doc_name}#{chunk_idx}] "
            f"(combined={it['combined']:.4f})\n{it['doc']}"
        )
    context = "\n\n---SECTION---\n\n".join(parts)
    return context, items

def get_llm_answer(chain, question: str, context: str) -> str:
    msg = chain.invoke({"context": context, "question": question})
    return getattr(msg, "content", str(msg))

def format_response(
    question: str, answer: str, retrieved_items,
    max_sources: int = 6,
) -> str:
    lines = []
    lines.append(f"Question: {question}\n")
    lines.append("Answer:\n" + answer.strip() + "\n")
    lines.append("Sources:")
    for i, it in enumerate(retrieved_items[:max_sources], 1):
        meta = it["meta"]
        doc_name = meta.get("doc_name", "unknown_doc")
        chunk_index = meta.get("chunk_index", "unknown_chunk")
        preview = it["doc"][:180].replace("\n", " ").strip() + "..."
        lines.append(
            f"{i}. [{doc_name}#{chunk_index}] combined={it['combined']:.4f} | "
            f"vector={it['vector'] if it['vector'] is not None else 'n/a'} | "
            f"bm25={it['bm25'] if it['bm25'] is not None else 'n/a'}\n"
            f"   {preview}"
        )
    return "\n".join(lines)

def enhanced_query_with_llm(
    engine: HybridSearchEngine, chain, question: str,
    n_results: int = 5, vector_k: int = 40, bm25_k: int = 40,
) -> str:
    context, items = retrieve_context(
        engine, question, n_results=n_results,
        vector_k=vector_k, bm25_k=bm25_k,
    )
    answer = get_llm_answer(chain, question, context)
    return format_response(question, answer, items)
