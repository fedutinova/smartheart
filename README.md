# SmartHeart EKG Analysis System

## Overview

SmartHeart анализирует ЭКГ с использованием OpenCV для предобработки. Система автоматически загружает изображения по URL, применяет комплексную предобработку и извлекает характеристики ЭКГ сигнала.

### 🔬 ЭКГ Предобработка изображений
- **Изменение размера**: Приведение к фиксированным размерам (800x600)
- **Grayscale**: Перевод в градации серого
- **Контраст**: Повышение контраста с помощью histogram equalization
- **Бинаризация**: Adaptive threshold для выделения сигнала
- **Морфология**: Erosion/dilation для удаления шума
- **Извлечение сигнала**: Поиск самой длинной линии как ЭКГ сигнала

### 📊 Анализ характеристик сигнала
- Длина сигнала (arc length)
- Ширина сигнала
- Диапазон амплитуды
- Базовая линия
- Стандартное отклонение
- Bounding box сигнала

## Установка и запуск

### Требования
- Go 1.26
- OpenCV 4.x
- Docker & Docker Compose

### Локальная разработка

1. **Установите OpenCV**:
```bash
# Ubuntu/Debian
sudo apt-get install libopencv-dev pkg-config

# macOS
brew install opencv pkg-config

# Alpine (для Docker)
apk add opencv-dev pkgconfig build-base
```

2. **Установите зависимости**:
```bash
# Проверьте системные зависимости
make check-deps

# Установите Go модули
go mod download
```

3. **Запустите сервисы**:
```bash
docker-compose up -d postgres redis localstack
```

4. **Запустите приложение**:
```bash
# Используя Makefile
make run

# Или напрямую
CGO_ENABLED=1 go run cmd/main.go
```

### Docker

```bash
# Сборка и запуск
docker-compose up --build

# Или только приложение
docker build -t smartheart .
docker run -p 8080:8080 smartheart
```

## API Использование

### 1. Отправка ЭКГ анализа

```bash
curl -X POST http://localhost:8080/v1/ekg/analyze \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "image_temp_url": "https://example.com/ekg.jpg",
    "notes": "Emergency room EKG"
  }'
```

### 2. Проверка статуса

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/v1/jobs/JOB_ID
```

### 3. Получение результатов

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/v1/requests/REQUEST_ID
```

## Конфигурация

### Переменные окружения

```bash
# EKG Processing
OPENAI_API_KEY=your_openai_key

# Storage
STORAGE_MODE=local  # или s3
LOCAL_STORAGE_DIR=./uploads

# Для настройки S3 хранилища см. S3_SETUP.md

# Queue
QUEUE_WORKERS=4
QUEUE_BUFFER=1024
JOB_MAX_DURATION=30s

# Database
DATABASE_URL=postgres://user:password@localhost:5432/smartheart

# Redis
REDIS_URL=redis://localhost:6379
```

## Поддерживаемые форматы

- **Изображения**: JPEG, PNG, GIF, WebP, BMP, TIFF
- **Документы**: PDF (с изображениями)
- **Максимальный размер**: 10MB
- **Таймаут загрузки**: 30 секунд

## Мониторинг и логи

Система предоставляет подробные логи:

```json
{
  "level": "info",
  "msg": "EKG analysis completed successfully",
  "job_id": "123e4567-e89b-12d3-a456-426614174000",
  "signal_length": 150.5,
  "processing_steps": ["resized", "grayscale", "contrast_enhanced", "binarized", "morphological_processed", "signal_extracted"]
}
```

## Производительность

- **Обработка изображения**: ~2-5 секунд
- **Параллельная обработка**: До 4 воркеров одновременно
- **Память**: ~50-100MB на изображение
- **CPU**: Оптимизировано для OpenCV операций

## Безопасность

- ✅ Валидация типов файлов
- ✅ Ограничение размера файлов
- ✅ Таймауты загрузки
- ✅ JWT аутентификация
- ✅ Ролевая авторизация
- ✅ Логирование всех операций

## Frontend

У SmartHeart есть веб-интерфейс на React + TypeScript.

### Запуск фронтенда

```bash
cd frontend

# Установить зависимости
npm install

# Запустить dev сервер
npm run dev

# Приложение будет доступно на http://localhost:3000
```

### Возможности интерфейса

- 🔐 Авторизация (регистрация, вход, выход)
- 📊 Анализ ЭКГ с загрузкой изображений
- 📈 Просмотр результатов с детальными характеристиками
- 📜 История всех анализов
- 🎨 Адаптивный дизайн

Подробнее см. [frontend/README.md](frontend/README.md)

## Troubleshooting

### OpenCV ошибки
```bash
# Проверьте установку OpenCV
pkg-config --modversion opencv4

# Убедитесь, что CGO включен
export CGO_ENABLED=1
```

### Docker проблемы
```bash
# Пересоберите образ
docker-compose build --no-cache app

# Проверьте логи
docker-compose logs app
```

### Производительность
- Увеличьте `QUEUE_WORKERS` для большего параллелизма
- Настройте `JOB_MAX_DURATION` для больших изображений
- Используйте SSD для локального хранилища

## Разработка

### Добавление новых алгоритмов

1. Расширьте `EKGPreprocessor` в `internal/ekg/preprocessor.go`
2. Добавьте новые шаги обработки
3. Обновите `ExtractSignalFeatures` для новых характеристик

### Тестирование

```bash
# Запуск тестов
go test ./...

# Тесты с покрытием
go test -cover ./...
```
