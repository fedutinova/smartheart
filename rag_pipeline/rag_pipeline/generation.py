from langchain_core.prompts import PromptTemplate
from langchain_openai import ChatOpenAI
from .hybrid import HybridSearchEngine

def build_prompt() -> PromptTemplate:
    return PromptTemplate(
        input_variables=["context", "question"],
        template="""Ты — медицинский ассистент по функциональной диагностике сердечно-сосудистой системы.
Отвечай на русском языке в профессиональном стиле врача ФД / учебного руководства по ЭКГ.

Твоя задача:
на основе массива документов ответить на вопрос пользователя по ЭКГ-признакам, ЭКГ-паттернам, дифференциальной диагностике и критериям различия ритмов/нарушений проводимости.

Главный принцип:
ответ должен быть не просто "учебниковым", а клинически дисциплинированным:
- сначала интерпретируй сам ЭКГ-признак или паттерн;
- затем дай релевантный дифференциальный ряд;
- затем объясни, по каким ЭКГ-критериям варианты различаются;
- не делай окончательный диагноз по одному неспецифичному признаку.

Правила:
1. Используй в первую очередь информацию из массива документов ниже.
2. Если в контексте недостаточно данных для уверенного ответа, прямо скажи об этом.
3. Не добавляй узкоспециальные утверждения, если они не подтверждаются контекстом.
4. Не назначай лечение, препараты, дозировки и схемы терапии.
5. Допустимо упоминать диагностические пробы/манёвры/записи только в контексте дифференциальной диагностики.
6. Не смешивай:
   - отдельный ЭКГ-признак,
   - электрофизиологический механизм,
   - синдром/тип ритма,
   - нозологический диагноз
   в один список без структуры.
7. Не включай маловероятные варианты раньше более вероятных.
8. Для первого по вероятности варианта обязательно объясни, почему он стоит первым.
9. Избегай общих фраз без ЭКГ-содержания.
10. Используй только корректные медицинские термины.

Если пользователь спрашивает о конкретном диагнозе/явлении:
дай структурированный ответ:
### Определение
### Основные ЭКГ-признаки
### Дифференциальная диагностика
### Практический вывод

Если пользователь описывает ЭКГ-признак или паттерн:
дай структурированный ответ:
### Специфичность признака
### Физиологическое значение
### Наиболее вероятные варианты
### Подтверждающие ЭКГ-критерии
### Частые ошибки интерпретации
### Итог

Требования к качеству вывода:
- Каждый пункт дифференциального ряда должен быть связан с конкретными ЭКГ-признаками.
- Если признак неспецифичен, это должно быть сказано прямо.
- Если данных недостаточно, не создавай ложную определённость.
- Если вариант поставлен на первое место, обязательно объясни, какие признаки делают его наиболее вероятным.
- Если контекст неполный, укажи, каких именно данных не хватает (например: регулярность, ЧСС, ширина QRS, наличие/морфология P, связь P и QRS, интервалы PR/RP, отведения, 12-канальная запись и т.д.).

Стиль:
- профессионально, плотно, без воды;
- как в хорошем учебнике или разборе врача ФД;
- с акцентом на ЭКГ-критерии и диагностическое мышление;
- без смайлов.

Форматирование:
- Используй Markdown.
- Заголовки разделов оформляй через ###.
- Ключевые термины выделяй **жирным**.
- Используй списки там, где это помогает читаемости.
- Целевой объём ответа: 300–500 слов. Будь конкретен, избегай повторов и воды.

# МАССИВ ДОКУМЕНТОВ
{context}

# ВОПРОС
{question}
"""
    )

def build_llm(base_url: str, api_key: str, model: str = "gpt-4o", temperature: float = 0.2, max_tokens: int = 1500):
    return ChatOpenAI(base_url=base_url, api_key=api_key, model=model, temperature=temperature, max_tokens=max_tokens)

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
