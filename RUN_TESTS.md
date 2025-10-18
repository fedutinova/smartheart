# Запуск тестов SmartHeart

## Быстрый старт

### 1. Настройка тестовых данных
```bash
go run ./testdata/setup.go
```

### 2. Запуск всех тестов
```bash
make test
```

### 3. Запуск только EKG тестов
```bash
make test-ekg
```

## Детальные команды

### Unit тесты
```bash
# EKG preprocessor тесты
go test -v ./internal/ekg/

# EKG handler тесты
go test -v ./internal/workers/

# HTTP handler тесты
go test -v ./internal/transport/http/
```

### Integration тесты
```bash
# EKG integration тесты
go test -v ./internal/workers/ -run Integration

# Все integration тесты
go test -v ./... -run Integration
```

### Benchmark тесты
```bash
# EKG benchmark тесты
go test -bench=. -benchmem ./internal/ekg/ ./internal/workers/

# Все benchmark тесты
make benchmark
```

### Тесты с покрытием
```bash
# Генерация отчета покрытия
make test-coverage

# Просмотр отчета
open coverage.html
```

### Тесты с race detection
```bash
make test-race
```

## Примеры тестовых запросов

### Валидный EKG запрос
```bash
curl -X POST http://localhost:8080/v1/ekg/analyze \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "image_temp_url": "http://example.com/test_ekg.jpg",
    "notes": "Patient EKG from emergency room"
  }'
```

### Проверка статуса задачи
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/v1/jobs/JOB_ID
```

### Получение результатов
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/v1/requests/REQUEST_ID
```

## Тестовые данные

### Создание тестовых изображений
```bash
go run ./testdata/setup.go
```

### Типы тестовых изображений
- **test_ekg.jpg** - Синтетический EKG сигнал
- **test_image.png** - Простое цветное изображение
- **large_image.jpg** - Большое изображение для тестирования лимитов
- **corrupted_image.jpg** - Поврежденное изображение для тестирования ошибок

### Тестовые пользователи
```bash
# Валидный пользователь
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "testpassword"
  }'

# Вход
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "testpassword"
  }'
```

## Отладка тестов

### Подробный вывод
```bash
go test -v -race ./internal/ekg/
```

### Отладка конкретного теста
```bash
go test -run TestNewEKGPreprocessor -v ./internal/ekg/
```

### Логирование в тестах
```bash
go test -v -logtostderr ./internal/ekg/
```

## Производительность

### Профилирование CPU
```bash
go test -cpuprofile=cpu.prof -bench=. ./internal/ekg/
go tool pprof cpu.prof
```

### Профилирование памяти
```bash
go test -memprofile=mem.prof -bench=. ./internal/ekg/
go tool pprof mem.prof
```

### Анализ покрытия
```bash
go test -coverprofile=coverage.out ./internal/ekg/
go tool cover -html=coverage.out -o coverage.html
```

## CI/CD

### Локальная симуляция CI
```bash
make ci-test
make ci-build
```

### Полный тест suite
```bash
make test-full
```

## Troubleshooting

### OpenCV ошибки
```bash
# Проверка установки OpenCV
pkg-config --modversion opencv4

# Переустановка зависимостей
go clean -modcache
go mod download
```

### Docker тесты
```bash
# Пересборка образа
docker-compose build --no-cache app

# Проверка логов
docker-compose logs app
```

### Проблемы с базой данных
```bash
# Перезапуск сервисов
docker-compose down
docker-compose up -d postgres redis localstack
```

## Полезные команды

### Очистка
```bash
make clean
```

### Форматирование кода
```bash
make fmt
```

### Линтинг
```bash
make lint
```

### Проверка кода
```bash
make vet
```

## Метрики тестирования

### Целевые показатели
- **Покрытие кода**: >80%
- **Время выполнения тестов**: <30 секунд
- **Память**: <100MB на тест
- **Race conditions**: 0

### Текущие показатели
```bash
make test-coverage
```

## Расширение тестов

### Добавление нового теста
1. Создайте файл `*_test.go` в соответствующем пакете
2. Добавьте тестовые функции с префиксом `Test`
3. Добавьте benchmark функции с префиксом `Benchmark`
4. Запустите тесты: `go test -v ./package/`

### Добавление тестовых данных
1. Добавьте данные в `testdata/test_images.go`
2. Обновите `testdata/test_examples.go`
3. Запустите `go run ./testdata/setup.go`

### Добавление integration тестов
1. Создайте файл `*_integration_test.go`
2. Используйте тег `//go:build integration`
3. Запустите: `go test -tags=integration ./...`
