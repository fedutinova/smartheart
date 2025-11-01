# Настройка S3 хранилища

Проект поддерживает два варианта S3 хранилища:
1. **LocalStack** (для разработки) - локальный эмулятор S3
2. **AWS S3** (для production) - настоящий AWS S3

## Вариант 1: LocalStack (для разработки)

LocalStack уже настроен в `docker-compose.yml` и автоматически создает бакет при запуске.

### Шаги настройки:

1. **Запустите сервисы с LocalStack:**
```bash
docker-compose up -d localstack
```

2. **Создайте файл `.env` в корне проекта:**
```bash
STORAGE_MODE=s3
S3_ENDPOINT=http://localhost:4566
S3_BUCKET=smartheart-files
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
S3_FORCE_PATH_STYLE=true
```

3. **Проверьте работу:**
```bash
# Проверьте, что LocalStack запущен
docker-compose ps localstack

# Проверьте бакет
docker exec smartheart_localstack awslocal s3 ls
```

4. **Запустите приложение:**
```bash
go run cmd/main.go
```

Система автоматически обнаружит LocalStack и будет использовать S3 хранилище.

## Вариант 2: AWS S3 (для production)

### Предварительные требования:

- Аккаунт AWS
- Созданный S3 bucket
- IAM пользователь с правами доступа к S3

### Шаги настройки:

#### 1. Создайте S3 bucket в AWS

```bash
# Через AWS CLI
aws s3 mb s3://smartheart-files --region us-east-1

# Или через AWS Console
# Перейдите в S3 → Create bucket → назовите "smartheart-files"
```

#### 2. Настройте bucket policy для публичного чтения (для GPT доступа)

Создайте bucket policy:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::smartheart-files/*"
    }
  ]
}
```

Или через AWS CLI:
```bash
cat > bucket-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicReadGetObject",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::smartheart-files/*"
    }
  ]
}
EOF

aws s3api put-bucket-policy --bucket smartheart-files --policy file://bucket-policy.json
```

#### 3. Настройте CORS (если нужно для фронтенда)

```bash
cat > cors-config.json << 'EOF'
{
  "CORSRules": [
    {
      "AllowedOrigins": ["*"],
      "AllowedHeaders": ["*"],
      "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
      "MaxAgeSeconds": 3000
    }
  ]
}
EOF

aws s3api put-bucket-cors --bucket smartheart-files --cors-configuration file://cors-config.json
```

#### 4. Создайте IAM пользователя

1. Перейдите в IAM → Users → Create user
2. Назовите пользователя `smartheart-s3-user`
3. Прикрепите политику с правами:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "s3:PutObject",
           "s3:GetObject",
           "s3:DeleteObject",
           "s3:ListBucket"
         ],
         "Resource": [
           "arn:aws:s3:::smartheart-files",
           "arn:aws:s3:::smartheart-files/*"
         ]
       }
     ]
   }
   ```
4. Создайте Access Key для пользователя
5. Сохраните Access Key ID и Secret Access Key

#### 5. Настройте переменные окружения

Создайте `.env` файл:
```bash
STORAGE_MODE=s3
S3_BUCKET=smartheart-files
S3_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key-id
AWS_SECRET_ACCESS_KEY=your-secret-access-key
S3_FORCE_PATH_STYLE=false
# S3_ENDPOINT не нужен для AWS S3 (оставьте пустым или не указывайте)
```

#### 6. Альтернатива: использование AWS credentials через AWS CLI

Если у вас настроен AWS CLI с профилем:
```bash
# Используйте стандартные переменные AWS
export AWS_PROFILE=your-profile
export AWS_REGION=us-east-1
```

И в `.env`:
```bash
STORAGE_MODE=s3
S3_BUCKET=smartheart-files
S3_REGION=us-east-1
# AWS_ACCESS_KEY_ID и AWS_SECRET_ACCESS_KEY будут взяты из AWS credentials
```

#### 7. Проверьте настройку

```bash
# Проверьте доступ к bucket
aws s3 ls s3://smartheart-files

# Запустите приложение
go run cmd/main.go
```

В логах должно появиться:
```
storage initialized type=AWS S3
```

## Переменные окружения для S3

| Переменная | Описание | Значение по умолчанию | Пример |
|-----------|----------|----------------------|--------|
| `STORAGE_MODE` | Режим хранилища | `local` | `s3`, `aws`, `localstack` |
| `S3_BUCKET` | Имя S3 bucket | `smartheart-files` | `my-bucket` |
| `S3_REGION` | AWS регион | `us-east-1` | `eu-west-1` |
| `S3_ENDPOINT` | Endpoint для S3 (только для LocalStack) | `http://localhost:4566` | `http://localstack:4566` |
| `AWS_ACCESS_KEY_ID` | AWS Access Key | `test` | Ваш ключ |
| `AWS_SECRET_ACCESS_KEY` | AWS Secret Key | `test` | Ваш секретный ключ |
| `S3_FORCE_PATH_STYLE` | Использовать path-style URLs | `true` | `true`/`false` |

## Проверка работоспособности

### Для LocalStack:
```bash
# Проверка бакета
docker exec smartheart_localstack awslocal s3 ls s3://smartheart-files

# Проверка через приложение
curl -X POST http://localhost:8080/v1/ekg/analyze \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image_temp_url": "https://example.com/test.jpg"}'
```

### Для AWS S3:
```bash
# Проверка через AWS CLI
aws s3 ls s3://smartheart-files/

# Проверка через приложение (тот же запрос)
```

## Важные замечания

1. **Публичный доступ**: Для работы с GPT API нужно настроить публичный доступ к файлам (bucket policy). OpenAI должен иметь возможность скачать изображение по URL.

2. **Безопасность**: В production используйте IAM роли вместо хранения ключей в переменных окружения. Для приложений на AWS (EC2, ECS, Lambda) используйте IAM roles.

3. **LocalStack vs AWS**: 
   - LocalStack - для разработки, все данные локальные
   - AWS S3 - для production, данные в облаке

4. **Presigned URLs**: Система автоматически генерирует presigned URLs для доступа к файлам. Для локального хранилища автоматически используется base64.

## Troubleshooting

### Ошибка "bucket does not exist"
- Убедитесь, что bucket создан
- Проверьте правильность имени в `S3_BUCKET`
- Проверьте права доступа IAM пользователя

### Ошибка "Access Denied"
- Проверьте IAM политику пользователя
- Убедитесь, что ключи правильные
- Проверьте bucket policy для публичного чтения

### Ошибка "Error while downloading" в GPT
- Для AWS S3: убедитесь, что bucket имеет публичный доступ
- Для LocalStack: система автоматически использует base64
- Проверьте CORS настройки bucket

### LocalStack не запускается
```bash
# Проверьте логи
docker-compose logs localstack

# Пересоздайте контейнер
docker-compose down
docker-compose up -d localstack
```

## Миграция с local на S3

Если вы уже используете local storage и хотите переключиться на S3:

1. Экспортируйте файлы из `./uploads` (если нужно сохранить данные)
2. Настройте S3 как описано выше
3. Измените `STORAGE_MODE=s3` в `.env`
4. Перезапустите приложение

Новые файлы будут загружаться в S3, старые останутся в local storage.

