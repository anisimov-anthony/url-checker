# URL Checker Service

Web server for checking internet resource availability with PDF report generation

## API Endpoints

### POST /api/check
Check link availability

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
Generate PDF report by batch numbers

**Request:**
```json
{
    "links_list": [1, 2]
}
```

**Response:** PDF file with report


### GET /api/health
Service health check

**Response:**
```json
{
    "status": "healthy",
    "shutdown": false,
    "batches": 5,
    "timestamp": 1765108565
}
```

## Installation and Running

### Requirements
- Go 1.21 or higher

### Install Dependencies
```bash
go mod tidy
```

### Running
```bash
go run main.go
```

The service will be available on port `8080`

### Check Links
```bash
curl -X POST http://localhost:8080/api/check \
  -H "Content-Type: application/json" \
  -d '{"links": ["google.com", "github.com", "nonexistent.example"]}'
```

### Request Report
```bash
curl -X POST http://localhost:8080/api/report \
  -H "Content-Type: application/json" \
  -d '{"links_list": [1, 2]}' \
  --output report.pdf
```

### Health Check
```bash
curl http://localhost:8080/api/health
```

## Testing

### Run Tests
```bash
./test-coverage.sh
```

### Code Coverage
Current coverage: **85.9%**
- **Database**: 82.4%
- **Service**: 89.3%  
- **Handlers**: 84.8%


## License

This project is distributed under the [MIT License](LICENSE).
