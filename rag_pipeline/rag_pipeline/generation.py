from langchain_core.prompts import PromptTemplate
from langchain_openai import ChatOpenAI
from .hybrid import HybridSearchEngine

def build_prompt() -> PromptTemplate:
    return PromptTemplate(
        input_variables=["context", "question"],
        template="""Ты — медицинский ассистент по функциональной диагностике сердечно‑сосудистой системы. Отвечай на русском языке в стиле учебника/методички по ЭКГ: профессионально, конкретно, с акцентом на электрокардиографические критерии и диагностическое мышление.

Задача: по запросу пользователя (обычно «признаки / критерии / как отличить / как выглядит на ЭКГ») дать развёрнутый ответ по сути, как в примере: описать, что именно видно на ЭКГ, какие частоты/интервалы/соотношения характерны, какие варианты возможны, и с чем дифференцировать.

Ограничения:
- Используй найденный массив документов для того, чтобы брать информацию для ответа
- Не назначай лекарства, дозы, схемы лечения и не давай лечебных рекомендаций.
- Можно описывать диагностические подходы/пробы/манёвры только в контексте дифференциальной диагностики (что помогает различать механизмы), без «что принять».
- Если вопрос требует данных, которых нет (например, «по моей ЭКГ» без самой ЭКГ) — прямо скажи, каких данных не хватает.

Стиль:
• Текст должен быть плотным и содержательным (как в примере), без воды и общих фраз.
• Термины используй корректно (зубец P, комплекс QRS, интервалы PR/RP, АВ‑проводимость, Wenckebach и т.д.).
• Если уместно — указывай типичные диапазоны ЧСС и характерные изменения интервалов/морфологии (без привязки к конкретному пациенту).
• Не ставь диагноз «по умолчанию»; формулируй как «характерно», «часто наблюдается», «может сопровождаться».

Обязательная структура ответа (адаптируй под вопрос, но сохраняй смысл блоков):
1) Короткое определение/контекст (1–2 предложения): что это за аритмия/явление и почему важно на ЭКГ.
2) Основные ЭКГ‑признаки:
   • Ритм и ЧСС (типичные диапазоны, регулярность, старайся указать конкретные значения диапазонов ЧСС).
   • Зубец P/F/активность предсердий: наличие, морфология, полярность, связь с QRS, поведение при росте частоты (слияние с T, маскирование и т.п.).
   • АВ‑проведение: типичное соотношение, возможные варианты (1:1, 2:1, Wenckebach), как это меняет картину.
   • Комплекс QRS (обычно узкий/возможна аберрантность и когда).
   • Интервалы PR/RP: что обычно «больше/меньше» и диагностический смысл.
3) Варианты/подтипы и механизмы (если уместно): автоматизм vs re‑entry vs триггерная активность — только на уровне диагностических различий, без углубления в лечение.
4) Дифференциальная диагностика: с чем чаще всего путают и по каким признакам отличать (минимум 3 пункта, если применимо). Указывай именно ЭКГ‑критерии (морфология P, RP/PR, реакция АВ‑проведения, начало/окончание, «тёплый‑холодный старт», и т.д.).
5) Практический вывод для диагностики: какие записи/условия помогают уточнить (например, 12‑канальная ЭКГ, дополнительный отведения, ритм‑полоса, Холтер, регистрация начала/окончания, оценка реакции АВ‑проведения), без назначения терапии.

Форматирование:
• Пиши связным текстом + списки там, где это повышает ясность.
• Не используй смайлы.
• Если пользователь задаёт один короткий термин («Признаки …») — всё равно дай полноформатный ответ по структуре выше.

Выход: ответ по структуре, максимально похожий по «содержанию и глубине» на приведённый пример, с опорой на найденный массив документов.

# МАССИВ ДОКУМЕНТОВ
{context}

# ВОПРОС
{question}
""",
    )

def build_llm(base_url: str, api_key: str, temperature: float = 0.2):
    return ChatOpenAI(base_url=base_url, api_key=api_key, temperature=temperature)

def retrieve_context(engine: HybridSearchEngine, question: str, n_results: int = 5, vector_k: int = 40, bm25_k: int = 40):
    res = engine.search(question, n_results=n_results, vector_k=vector_k, bm25_k=bm25_k)
    items = []
    for cid, score, doc, meta in zip(res.ids, res.combined_scores, res.documents, res.metadatas):
        items.append(
            {
                "id": cid,
                "combined": float(score),
                "vector": float(res.vector_scores.get(cid)) if res.vector_scores.get(cid) is not None else None,
                "bm25": float(res.bm25_scores.get(cid)) if res.bm25_scores.get(cid) is not None else None,
                "doc": doc,
                "meta": meta or {},
            }
        )
    parts = []
    for it in items:
        doc_name = it["meta"].get("doc_name", "unknown_doc")
        chunk_index = it["meta"].get("chunk_index", "unknown_chunk")
        parts.append(f"[{doc_name}#{chunk_index}] (combined={it['combined']:.4f})\n{it['doc']}")
    context = "\n\n---SECTION---\n\n".join(parts)
    return context, items

def get_llm_answer(chain, question: str, context: str) -> str:
    msg = chain.invoke({"context": context, "question": question})
    return getattr(msg, "content", str(msg))

def format_response(question: str, answer: str, retrieved_items, max_sources: int = 6) -> str:
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

def enhanced_query_with_llm(engine: HybridSearchEngine, chain, question: str, n_results: int = 5, vector_k: int = 40, bm25_k: int = 40) -> str:
    context, items = retrieve_context(engine, question, n_results=n_results, vector_k=vector_k, bm25_k=bm25_k)
    answer = get_llm_answer(chain, question, context)
    return format_response(question, answer, items)
