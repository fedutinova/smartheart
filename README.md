# SmartHeart EKG Analysis System

## Overview

SmartHeart — система анализа ЭКГ изображений с использованием OpenCV для предобработки и GPT для интерпретации результатов. Включает Go-бэкенд, React-фронтенд и инфраструктуру на Docker Compose (PostgreSQL, Redis, S3/LocalStack).

## Архитектура

```
frontend/          React + TypeScript + Vite
back-api/          Go (chi router, pgx, Redis)
  ├── auth/        JWT аутентификация, RBAC, middleware
  ├── config/      Конфигурация из env
  ├── handler/     HTTP-обработчики, OpenAPI спецификация
  ├── notify/      SSE-уведомления (per-user hub)
  ├── repository/  PostgreSQL data access
  ├── service/     Бизнес-логика (auth, submission, request)
  ├── workers/     Фоновые обработчики (EKG, GPT)
  ├── queue/       Очередь задач (Redis / in-memory)
  └── storage/     Хранилище файлов (S3 / local)
rag_pipeline/      Python RAG-сервис (FastAPI)
  ├── api/         FastAPI-сервер (/query)
  ├── rag_pipeline/
  │   ├── ingestion.py    Загрузка и индексация документов
  │   ├── hybrid.py       Гибридный поиск (vector + BM25 + RRF)
  │   ├── generation.py   LLM-генерация ответов
  │   ├── chunking.py     Семантическое разбиение текста
  │   └── tokenization.py Русский медицинский токенизатор
  └── documents/   Медицинские PDF (кардиология, ЭКГ)
migrations/        SQL-миграции
```

### Обработка ЭКГ

1. Пользователь загружает изображение (файл или URL) через фронтенд
2. Бэкенд ставит задачу в очередь (Redis stream)
3. EKG worker: предобработка OpenCV (resize → grayscale → contrast → binarization → morphology → signal extraction)
4. GPT worker: интерпретация результатов через OpenAI API
5. SSE-уведомление отправляется пользователю в реальном времени

### Характеристики сигнала
- Длина сигнала (arc length), ширина, диапазон амплитуды
- Базовая линия, стандартное отклонение, bounding box

## Установка и запуск

### Требования
- Go 1.26+, OpenCV 4.x, Node.js
- Docker & Docker Compose

### Быстрый старт

```bash
# 1. Инфраструктура
docker-compose up -d postgres redis localstack

# 2. Бэкенд
go mod download
CGO_ENABLED=1 go run cmd/main.go

# 3. Фронтенд
cd frontend && npm install && npm run dev
```

Приложение: бэкенд на `http://localhost:8080`, фронтенд на `http://localhost:5173`.

### Docker

```bash
docker-compose up --build
```

## API

### OpenAPI спецификация

Полная спецификация доступна по адресу:
```
GET /openapi.yaml
```
Описывает все эндпоинты, схемы запросов/ответов и коды ошибок (OpenAPI 3.0.3).

### Аутентификация

JWT-токены (access + refresh). Access-токен передается в заголовке `Authorization: Bearer <token>`.

```bash
# Регистрация
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepass"}'

# Вход
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepass"}'

# Обновление токена
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "REFRESH_TOKEN"}'

# Выход
curl -X POST http://localhost:8080/v1/auth/logout \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "REFRESH_TOKEN"}'
```

### ЭКГ анализ

Поддерживает два режима: загрузка файла (multipart) и отправка URL (JSON).

```bash
# Загрузка файла (рекомендуется)
curl -X POST http://localhost:8080/v1/ekg/analyze \
  -H "Authorization: Bearer TOKEN" \
  -F "image=@ekg.jpg" \
  -F "notes=Описание пациента"

# По URL
curl -X POST http://localhost:8080/v1/ekg/analyze \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image_temp_url": "https://example.com/ekg.jpg", "notes": "Описание"}'
```

### GPT обработка

```bash
curl -X POST http://localhost:8080/v1/gpt/process \
  -H "Authorization: Bearer TOKEN" \
  -F "text_query=Проанализируй ЭКГ" \
  -F "files=@image.jpg"
```

### Запросы и результаты

```bash
# Статус задачи
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/jobs/JOB_ID

# Результат запроса
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/requests/REQUEST_ID

# История запросов (с пагинацией)
curl -H "Authorization: Bearer TOKEN" "http://localhost:8080/v1/requests?limit=20&offset=0"
```

### SSE уведомления

Уведомления о завершении анализа в реальном времени через Server-Sent Events:

```bash
curl -N -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/events
```

Формат событий:
```json
{"type": "request_completed", "request_id": "uuid", "status": "completed"}
{"type": "request_failed", "request_id": "uuid", "status": "failed"}
```

> EventSource API не поддерживает заголовки — фронтенд передает токен через query-параметр `?token=`.

### RAG — Чат-бот по ЭКГ

Вопросно-ответная система на основе медицинской литературы. Гибридный поиск (vector + BM25) + LLM-генерация.

```bash
curl -X POST http://localhost:8080/v1/rag/query \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"question": "Признаки фибрилляции предсердий на ЭКГ"}'
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

### Health check

```bash
GET /health    # Публичный, для load balancer
GET /ready     # Защищённый (admin), проверяет DB + Redis + Storage
```

## Конфигурация

Переменные окружения (файл `.env` или `.env.local`):

| Переменная | По умолчанию | Описание |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `DATABASE_URL` | `postgres://...localhost:5432/smartheart` | PostgreSQL |
| `REDIS_URL` | `redis://localhost:6379` | Redis |
| `OPENAI_API_KEY` | — | Ключ OpenAI API |
| `GPT_MODEL` | `gpt-4o` | Модель GPT |
| `JWT_SECRET` | dev-default | Секрет JWT (обязателен в production) |
| `JWT_TTL_ACCESS` | `15m` | Время жизни access-токена |
| `JWT_TTL_REFRESH` | `168h` | Время жизни refresh-токена (7 дней) |
| `STORAGE_MODE` | `local` | Режим хранилища: `local`, `s3`, `aws` |
| `LOCAL_STORAGE_DIR` | `./uploads` | Директория для локального хранилища |
| `QUEUE_MODE` | `redis` | Очередь: `redis` или `memory` |
| `QUEUE_WORKERS` | `4` | Количество воркеров |
| `QUEUE_BUFFER` | `1024` | Размер буфера очереди |
| `JOB_MAX_DURATION` | `30s` | Таймаут обработки задачи |
| `QUOTA_DAILY_LIMIT` | `50` | Лимит запросов на пользователя в день (0 = без лимита) |
| `RATE_LIMIT_RPM` | `100` | Rate limit запросов в минуту на IP |
| `CORS_ORIGINS` | `localhost:3000,localhost:5173` | Разрешённые CORS origins |

## Frontend

React + TypeScript + Vite + TailwindCSS + React Query.

### Возможности

- Авторизация (регистрация, вход, автообновление токенов)
- Загрузка ЭКГ с обрезкой изображения (react-image-crop)
- Просмотр результатов с GPT-интерпретацией (Markdown)
- История анализов с пагинацией
- SSE-уведомления о завершении обработки
- Чат-бот по ЭКГ
- Адаптивный дизайн (мобильное меню)
- Lazy loading роутов, Error Boundary
- Контактная страница

### Запуск

```bash
cd frontend
npm install
npm run dev       # Dev-сервер (http://localhost:5173)
npm run build     # Production-сборка
```

## Безопасность

- JWT аутентификация с blacklist (logout invalidation)
- Ролевая авторизация (RBAC) с пермишенами
- Rate limiting по IP
- Квоты на количество запросов в день
- Валидация типов и размеров файлов
- CORS, Security headers (CSP, X-Frame-Options, etc.)
- Structured logging (slog)

## Поддерживаемые форматы

- **Изображения**: JPEG, PNG, GIF, WebP, BMP, TIFF
- **Документы**: PDF (с изображениями)
- **Максимальный размер**: 10MB

## Разработка

### Тестирование

```bash
go test ./...              # Все тесты
go test -cover ./...       # С покрытием
cd frontend && npx tsc --noEmit  # TypeScript проверка
cd frontend && npx vite build    # Frontend сборка
```

### Генерация моков

```bash
# Требует mockery v2.52+
$(go env GOPATH)/bin/mockery
```

### Миграции

SQL-миграции находятся в `migrations/`. Применяются при старте приложения.

### OpenCV

```bash
# Ubuntu/Debian
sudo apt-get install libopencv-dev pkg-config

# macOS
brew install opencv pkg-config

# Проверка
pkg-config --modversion opencv4
```
