# Production-Ready HTTP Service Example

This example demonstrates how to build a production-ready HTTP service using SafeHeaders-Go modules.

## Features

- ✅ **HTTP Server** with graceful shutdown
- ✅ **Input Validation** and size limits
- ✅ **Rate Limiting** middleware
- ✅ **Request Logging** and monitoring
- ✅ **Health Checks** with memory stats
- ✅ **Context Timeouts** for request processing
- ✅ **Error Handling** with proper HTTP status codes
- ✅ **Security Best Practices** (request size limits, timeouts)

## Running the Server

```bash
go run main.go
```

The server will start on `http://localhost:8080`.

## API Endpoints

### POST /api/parse

Parse JSON data.

**Request:**
```bash
curl -X POST http://localhost:8080/api/parse \
  -H "Content-Type: application/json" \
  -d '{"name": "test", "value": 123}'
```

**Response:**
```json
{
  "status": "success",
  "tokenCount": 5,
  "inputSize": 32,
  "parseTime": "123.45µs",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

### GET /health

Health check endpoint.

**Request:**
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-01T00:00:00Z",
  "memory": {
    "allocMB": 5,
    "totalMB": 10,
    "sysMB": 20,
    "numGC": 3,
    "goroutines": 5
  }
}
```

### GET /stats

Server statistics.

**Request:**
```bash
curl http://localhost:8080/stats
```

**Response:**
```json
{
  "requestCount": 42,
  "cpus": 8,
  "goVersion": "go1.23.0",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

## Configuration

Edit the constants in `main.go`:

```go
const (
    ServerPort     = ":8080"           // Server address
    ReadTimeout    = 10 * time.Second  // Request read timeout
    WriteTimeout   = 10 * time.Second  // Response write timeout
    MaxRequestSize = 10 * 1024 * 1024  // 10MB request limit
    ParseTimeout   = 30 * time.Second  // JSON parse timeout
)
```

## Deployment

### Build for Production

```bash
go build -ldflags="-s -w" -o server main.go
```

### Run with Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -ldflags="-s -w" -o server main.go

FROM alpine:latest
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
```

### Environment Variables

```bash
export PORT=8080
export MAX_REQUEST_SIZE=10485760
./server
```

## Monitoring

Monitor the service using the `/health` and `/stats` endpoints:

```bash
# Check health every 30 seconds
watch -n 30 curl -s http://localhost:8080/health | jq

# Monitor request count
watch -n 5 curl -s http://localhost:8080/stats | jq .requestCount
```

## Load Testing

Test with `ab` (Apache Bench):

```bash
# 1000 requests, 10 concurrent
ab -n 1000 -c 10 -p test.json -T application/json \
  http://localhost:8080/api/parse
```

Or with `hey`:

```bash
hey -n 1000 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}' \
  http://localhost:8080/api/parse
```

## Security Considerations

This example implements several security best practices:

1. **Request Size Limits** - Prevents DoS via large payloads
2. **Timeouts** - Prevents slow client attacks
3. **Context Cancellation** - Allows graceful shutdown
4. **Input Validation** - Rejects invalid requests
5. **Graceful Shutdown** - Finishes in-flight requests

For production, also consider:
- HTTPS/TLS encryption
- Authentication/Authorization
- Rate limiting per client IP
- DDoS protection
- Security headers (HSTS, CSP, etc.)
- Logging to external service

## License

MIT License - Same as SafeHeaders-Go project
