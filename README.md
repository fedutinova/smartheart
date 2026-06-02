# Умное сердце

## Обзор

Умное сердце — сервис для анализа ЭКГ по фотографиям бумажных плёнок. Пользователь загружает снимок, изображение обезличивается прямо в браузере (Tesseract.js + русскоязычная BERT-NER модель `bert-base-NER-Russian`), затем параллельно запускаются:
- vision-LLM (GPT-4o) — извлекает измерения зубцов и интервалов по сетке;
- отдельный ML-сервис (`cv_service`) — классифицирует ритм и выявляет признаки гипертрофии;
- vision-LLM ещё раз — формирует медицинское объяснение по предсказанию ритма.

Серверная часть на Go (chi, pgx), веб-интерфейс на React + TypeScript + Vite, отдельная админ-панель, инфраструктура — Docker Compose (PostgreSQL, Redis, S3/LocalStack). Оплата подписки через YooKassa.

## Архитектура

```
frontend/          Веб-интерфейс пользователя (React + TypeScript + Vite)
admin/             Админ-панель (отдельное SPA)
back-api/          Серверная часть на Go (chi, pgx, Redis)
  ├── auth/        JWT, ролевая модель, middleware
  ├── config/      Конфигурация из переменных окружения
  ├── cv/          HTTP-клиент к cv_service
  ├── gpt/         Клиент OpenAI + промпты ЭКГ
  ├── handler/     HTTP-обработчики, спецификация OpenAPI
  ├── job/         Описание заданий и реестр обработчиков
  ├── notify/      SSE-хаб уведомлений (один экземпляр на пользователя)
  ├── queue/       Очередь заданий (Redis либо in-memory)
  ├── redaction/   Серверная реализация band/OCR-маскирования (используется для H2)
  ├── repository/  Доступ к данным PostgreSQL
  ├── service/     Бизнес-логика (auth, submission, payments, request)
  ├── storage/     Хранилище файлов (S3 или локальная директория)
  └── workers/     Фоновые обработчики ЭКГ и GPT-задач
cv_service/        Python-сервис классификации ритма (FastAPI + PyTorch)
rag_pipeline/      Вопросно-ответный сервис по медицинской литературе (Python + FastAPI)
migrations/        SQL-миграции, применяются при старте серверной части
h2/                Артефакты гипотезы H2 (датасет, скрипты проверки)
```

### Поток анализа ЭКГ

1. На странице `/analyze` пользователь выбирает файл с устройства или делает фото камерой.
2. Браузер запускает локальный OCR-конвейер (Tesseract.js + ruBERT NER + регулярные выражения) и закрашивает обнаруженные идентификаторы (ФИО, даты, СНИЛС, ID пациента). В серверную часть уходит уже обезличенный снимок плюс метрики маскирования (`client_meta`).
3. Серверная часть принимает multipart-запрос на `POST /v1/ecg/analyze`, проверяет квоту/подписку, сохраняет файл в S3 и ставит задание в очередь.
4. Фоновый обработчик ЭКГ параллельно вызывает GPT-4o (измерения по сетке) и `cv_service` (классификация ритма + бинарные признаки). При успехе классификатора повторный vision-вызов формирует медицинский текст.
5. Постобработка переводит количество клеток в мВ и мс, считает индексы (Соколов-Лайон, Корнельский, Пегеро-Ло Прести, Губнер, Льюис), классифицирует ось.
6. Результат сохраняется в БД, пользователю приходит SSE-событие `request_completed`.

### Гипотеза H2 (приватность)

Сравнение точечного OCR-маскирования с маскированием типовых зон. Реализация — на фронтенде (`frontend/src/utils/redaction.ts`, `piiDetector.ts`); серверный эндпоинт `POST /v1/ecg/analyze-h2-compare` запускает обе модели на одной картинке и возвращает метрики (`masked_area_ratio`, `boxes_count`, время выполнения). Артефакты и сценарии проверки — в каталоге `h2/`.

## Установка и запуск

### Требования

- Go 1.26+
- Node.js 20+
- Docker и Docker Compose
- Python 3.11+ (для `cv_service` и `rag_pipeline`)

### Быстрый старт

```bash
# 1. Инфраструктура
docker compose up -d postgres redis localstack

# 2. Серверная часть
go mod download
CGO_ENABLED=0 go run ./cmd

# 3. Веб-интерфейс
cd frontend && npm install && npm run dev
```

Адреса: серверная часть — `http://localhost:8080`, веб-интерфейс — `http://localhost:3000`.

### Сборка через Docker Compose

```bash
docker compose up --build
```

## API

### Спецификация OpenAPI

Полное описание доступно по адресу `GET /openapi.yaml` (стандарт OpenAPI 3.0.3). В нём перечислены все маршруты, схемы запросов/ответов и коды ошибок.

### Аутентификация

JWT с парой access/refresh. Access передаётся в заголовке `Authorization: Bearer <token>`.

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

### Анализ ЭКГ

Единственный способ загрузки — multipart-форма с уже обезличенным изображением и опциональными метриками клиентского маскирования в поле `client_meta`.

```bash
curl -X POST http://localhost:8080/v1/ecg/analyze \
  -H "Authorization: Bearer TOKEN" \
  -F "image=@ekg.jpg" \
  -F "age=58" \
  -F "sex=male" \
  -F "paper_speed_mms=25" \
  -F "layout_label=3x4_rhythm" \
  -F 'client_meta={"redaction_mode":"ocr","redaction_ms":1820,"boxes_count":4,"masked_area_ratio":0.07,"image_width":2480,"image_height":1748}'
```

### Запросы и результаты

```bash
# Статус задания
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/jobs/JOB_ID

# Результат запроса
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/requests/REQUEST_ID

# История запросов (с постраничным выводом)
curl -H "Authorization: Bearer TOKEN" "http://localhost:8080/v1/requests?limit=20&offset=0"
```

### SSE-уведомления

Сообщения о завершении обработки доставляются через Server-Sent Events:

```bash
curl -N -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/events
```

Формат событий:
```json
{"type": "request_completed", "request_id": "uuid", "status": "completed"}
{"type": "request_completed", "request_id": "uuid", "status": "failed"}
```

> EventSource API браузера не поддерживает пользовательские заголовки — веб-интерфейс передаёт токен через параметр запроса `?token=`.

### Контекстный чат по ЭКГ

После завершения анализа пользователь может задавать вопросы по конкретному снимку. Контекст (измерения, индексы, классификация ритма) подмешивается к промпту автоматически.

```bash
curl -X POST http://localhost:8080/v1/ecg/REQUEST_ID/chat/messages \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"text": "Что означает индекс Соколова-Лайона выше нормы?"}'
```

### RAG — вопросно-ответная система по литературе

Гибридный поиск (векторный + BM25 + RRF) по медицинским источникам с генерацией ответа LLM.

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

### Квоты и оплата

```bash
# Остаток бесплатных анализов и статус подписки
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/quota

# История платежей
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/v1/payments
```

Платежи проходят через YooKassa; уведомление о смене статуса приходит на `POST /v1/payments/webhook` и дополнительно проверяется обратным запросом к YooKassa.

### Проверки состояния

```bash
GET /health    # публичная, для балансировщика нагрузки
GET /ready     # для админов: проверяет PostgreSQL, Redis и хранилище
```

## Конфигурация

Переменные окружения (файлы `.env` или `.env.local`):

| Переменная | По умолчанию | Описание |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `DATABASE_URL` | `postgres://...localhost:5432/smartheart` | Строка подключения к PostgreSQL |
| `REDIS_URL` | `redis://localhost:6379` | Строка подключения к Redis |
| `OPENAI_API_KEY` | — | Ключ доступа к OpenAI |
| `GPT_MODEL` | `gpt-4o` | Модель vision-LLM |
| `CV_URL` | — | Адрес `cv_service` (если пусто — классификация ритма отключена) |
| `JWT_SECRET` | значение по умолчанию для разработки | Секрет JWT (обязателен в production) |
| `JWT_TTL_ACCESS` | `15m` | Время жизни access-токена |
| `JWT_TTL_REFRESH` | `168h` | Время жизни refresh-токена (7 дней) |
| `STORAGE_MODE` | `local` | Режим хранилища: `local`, `s3`, `aws` |
| `LOCAL_STORAGE_DIR` | `./uploads` | Каталог локального хранилища |
| `QUEUE_MODE` | `redis` | Очередь: `redis` либо `memory` |
| `QUEUE_WORKERS` | `4` | Количество фоновых обработчиков |
| `QUEUE_BUFFER` | `1024` | Размер буфера очереди |
| `JOB_MAX_DURATION` | `30s` | Тайм-аут обработки одного задания |
| `QUOTA_FREE_LIMIT` | `3` | Сколько бесплатных анализов на пользователя за всё время (0 — без лимита) |
| `RATE_LIMIT_RPM` | `100` | Лимит частоты запросов в минуту с одного IP |
| `CORS_ORIGINS` | `localhost:3000,localhost:5173` | Разрешённые источники CORS |
| `YOOKASSA_SHOP_ID` | — | Идентификатор магазина YooKassa (пусто — режим разработки без оплаты) |
| `YOOKASSA_SECRET_KEY` | — | Секрет YooKassa для проверки webhook'ов |
| `YOOKASSA_SUBSCRIPTION_PRICE_KOPECKS` | — | Стоимость подписки в копейках |

## Веб-интерфейс

React + TypeScript + Vite + TailwindCSS + React Query.

### Возможности

- Авторизация (регистрация, вход, автообновление токенов).
- Загрузка ЭКГ с обрезкой кадра и встроенным OCR-маскированием идентификаторов прямо в браузере.
- Превью результата маскирования с возможностью пересоздать кадр или заменить файл.
- Просмотр структурированного результата: измерения по отведениям, индексы гипертрофии (Соколов-Лайон, Корнельский, Пегеро-Ло Прести, Губнер, Льюис), классификация оси, классификация ритма с медицинским объяснением.
- Контекстный чат по конкретному анализу.
- История анализов с постраничным выводом.
- SSE-уведомления о завершении обработки.
- Вопросно-ответный чат по медицинской литературе.
- Подписка через YooKassa, экран с историей платежей.
- Адаптивная вёрстка (отдельное мобильное меню), ленивая загрузка маршрутов, обработка ошибок через ErrorBoundary.

### Запуск

```bash
cd frontend
npm install
npm run dev       # сервер разработки (http://localhost:3000)
npm run build     # production-сборка
```

## Безопасность

- JWT с чёрным списком отозванных токенов (инвалидация при выходе).
- Ролевая модель с разрешениями (`auth.PermJobReadOwn`, `auth.PermAdminAll` и др.).
- Лимит частоты запросов по IP.
- Квоты на бесплатные анализы; платная подписка снимает ограничение.
- Проверка типов и размеров загружаемых файлов.
- CORS, security-заголовки (CSP, X-Frame-Options и т.д.).
- Структурированное логирование (`slog`).
- Клиентское обезличивание ЭКГ до отправки на сервер — персональные данные не покидают устройство пользователя.

## Поддерживаемые форматы

- Изображения: JPEG, PNG, GIF, WebP, BMP, TIFF.
- Документы: PDF (одностраничные).
- Максимальный размер: 10 МБ.

## Разработка

### Тестирование

```bash
make test                              # полный прогон (серверная часть, RAG, фронтенд, админка)
make test-backend                      # тесты серверной части без интеграционных
make test-backend-integration          # интеграционные тесты обработчика ЭКГ
make test-frontend                     # линт, тесты, production-сборка фронтенда
make verify-h2-ground-truth            # проверка H2 на размеченном датасете
```

### Генерация моков

```bash
# Требуется mockery v2.52+
$(go env GOPATH)/bin/mockery
```

Файл конфигурации — `.mockery.yaml`. Регенерировать моки нужно после изменения интерфейсов в `repository/`, `service/`, `storage/`, `cv/`, `job/`, `auth/`.

### Миграции

SQL-миграции лежат в каталоге `migrations/` и применяются при старте серверной части. Порядок — по числовому префиксу имени файла.

### Запуск cv_service

```bash
cd cv_service
pip install -r requirements.txt
# Положить файл модели в cv_service/models/stage_oldv9_residual_best.pt
uvicorn src.main:app --port 8001
```

Подробности — в [cv_service/README.md](cv_service/README.md).
