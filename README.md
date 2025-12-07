# URL Checker Service

Веб-сервер для проверки доступности интернет-ресурсов с генерацией PDF отчетов.

## API Эндпоинты

### POST /api/check
Проверка доступности ссылок

**Request:**
```json
{
    "links": ["google.com", "malformedlink.gg"]
}
```

**Response:**
```json
{
    "links": {
        "google.com": "available",
        "malformedlink.gg": "not available"
    },
    "links_num": 1
}
```

### POST /api/report
Генерация PDF отчета по номерам пакетов

**Request:**
```json
{
    "links_list": [1, 2]
}
```

**Response:** PDF файл с отчетом


### GET /api/health
Проверка здоровья сервиса

**Response:**
```json
{
    "status": "healthy",
    "shutdown": false,
    "batches": 5,
    "timestamp": 1701234567
}
```

## Установка и запуск

### Требования
- Go 1.21 или выше

### Установка зависимостей
```bash
go mod tidy
```

### Запуск
```bash
go run main.go
```

Сервис будет доступен на порту 8080.

### Проверка ссылок
```bash
curl -X POST http://localhost:8080/api/check \
  -H "Content-Type: application/json" \
  -d '{"links": ["google.com", "github.com", "nonexistent.example"]}'
```

### Запрос отчета
```bash
curl -X POST http://localhost:8080/api/report \
  -H "Content-Type: application/json" \
  -d '{"links_list": [1, 2]}' \
  --output report.pdf
```

### Проверка здоровья
```bash
curl http://localhost:8080/api/health
```

