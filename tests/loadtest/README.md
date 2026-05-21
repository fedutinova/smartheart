# H3 Load Test: Семантический кэш клинической базы знаний

Верификация гипотезы H3: сервер-сайд кэш с гибридным поиском (векторный + лексический) + фильтры безопасности обеспечивает hit rate ≥ 30% на held-out наборе при false-hit rate < 10% на негативном наборе.

## Зафиксированные результаты

Результаты финального прогона (v3, 2026-05-08) хранятся в репозитории и доступны без запуска теста:

- `results/h3_20260508_183928_interpretation.md` — интерпретация с выводами
- `results/h3_20260508_183928_*_summary.json` — метрики k6 по каждой фазе
- `results/h3_20260508_183928_*_k6.log` — полные логи

**Итог:** H3 подтверждена. Measure hit rate = 0.52 ✓, negative false-hit rate = 0.08 ✓, cached avg latency = 1 052 ms vs 7 901 ms (miss).

## Воспроизведение

### Предварительные условия

- Docker и Docker Compose
- OpenAI API ключ (для LLM-судьи, модель `gpt-4o-mini`)
- k6 (запускается через Docker, установка не требуется)

### Запуск

```bash
# 1. Поднять стек
docker compose up -d

# 2. Установить OpenAI ключ
export OPENAI_API_KEY=sk-...

# 3. Запустить полный тест (fill + measure + negative)
./tests/loadtest/run_h3.sh

# Или только minimal (fill + negative, для проверки guard'а)
./tests/loadtest/run_h3.sh --minimal
```

Результаты сохраняются в `tests/loadtest/results/` с временно́й меткой.

### Важно

LLM-судья (`gpt-4o-mini`, T=0) детерминирован, но OpenAI не гарантирует полную идентичность выводов при разных прогонах. Незначительные расхождения в hit rate (±1–2 запроса) возможны. Итоговые критерии (hit rate > 0.30, false-hit < 0.10) устойчивы к таким флуктуациям.

## Датасет

| Файл | Назначение | Записей |
|---|---|---:|
| `data/questions_h3_fill.json` | Заполнение кэша (25 канонических вопросов) | 25 |
| `data/questions_h3_heldout.json` | Измерение hit rate (новые формулировки тех же тем) | 50 |
| `data/questions_h3_negative.json` | Измерение false-hit rate (семантически близкие, но другие вопросы) | 25 |

## Анализ результатов из БД

После прогона — дополнительная аналитика через SQL:

```bash
docker exec smartheart_postgres psql -U user -d smartheart -f /tests/loadtest/h3_metrics.sql
```
