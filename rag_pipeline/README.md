# RAG Pipeline

Гибридный поиск (vector + BM25 + RRF) по медицинским документам с LLM-генерацией ответов.

## Структура

```
rag_pipeline/
  __init__.py
  config.py           # пути, параметры чанкинга, Chroma, эмбеддинги
  text_clean.py       # очистка/нормализация текста из PDF
  chunking.py         # семантический чанкинг с учётом заголовков
  tokenization.py     # русский медицинский токенизатор для BM25
  bm25.py             # BM25Okapi индекс и поиск
  hybrid.py           # HybridSearchEngine (vector + BM25 + RRF fusion)
  ingestion.py        # загрузка документов, чанкинг, индексация в ChromaDB
  generation.py       # промпт, LLM-клиент, RAG-генерация ответа
api/
  main.py             # FastAPI-сервер (/query, /health)
documents/            # медицинские PDF (не коммитятся)
handover_demo.py      # демо-скрипт: индексация → поиск → LLM-ответ
```

## Запуск (Docker)

Сервис запускается через `docker-compose` из корня проекта:

```bash
docker-compose up --build rag
```

Переменные окружения:

| Переменная | По умолчанию | Описание |
|---|---|---|
| `LLM_API_KEY` | — | API-ключ LLM-провайдера (обязателен) |
| `LLM_BASE_URL` | `https://bothub.chat/api/v2/openai/v1` | Base URL OpenAI-совместимого API |
| `LLM_TEMPERATURE` | `0.2` | Температура генерации |

## Запуск (локально)

```bash
cd rag_pipeline
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

# Поместите PDF в documents/
export LLM_API_KEY="your-key"

# API-сервер
uvicorn api.main:app --host 0.0.0.0 --port 8000

# Или демо-скрипт (индексация + поиск + ответ)
python handover_demo.py
```

## API

### `POST /query`

```bash
curl -X POST http://localhost:8000/query \
  -H "Content-Type: application/json" \
  -d '{"question": "Признаки фибрилляции предсердий на ЭКГ", "n_results": 5}'
```

Ответ:
```json
{
  "answer": "Фибрилляция предсердий характеризуется...",
  "sources": [
    {"doc_name": "Азбука_ЭКГ.pdf", "chunk_index": 42, "score": 0.0312, "preview": "..."}
  ],
  "elapsed_ms": 3200
}
```

### `GET /health`

```json
{"status": "ok"}
```

## Конфигурация пайплайна

Параметры в `rag_pipeline/config.py`:

- **Чанкинг**: `CHUNK_TARGET_CHARS=1900`, `CHUNK_MIN_CHARS=1400`, `CHUNK_MAX_CHARS=2600`, `CHUNK_OVERLAP_CHARS=250`
- **ChromaDB**: `CHROMA_PATH=./chroma_db_4`, `COLLECTION_NAME=cardio_docs_hybrid`, `HNSW_SPACE=cosine`
- **Эмбеддинги**: `intfloat/multilingual-e5-base`

## Переиндексация

Удалите папку ChromaDB и перезапустите сервис:

```bash
rm -rf chroma_db_4
# При следующем запуске индекс будет пересоздан автоматически
```
