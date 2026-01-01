# AdsTxt Analysis API

A production-ready RESTful API service for analyzing ads.txt files from content publisher websites. Built with Go, featuring concurrent processing, caching, rate limiting, and comprehensive logging.

## 📖 Table of Contents

- [Features](#-features)
- [API Endpoints](#-api-endpoints)
- [Architecture](#️-architecture)
- [Implementation Details](#-implementation-details)
  - [Complete Request Flow](#complete-request-flow)
  - [Request Lifecycle & Context Propagation](#request-lifecycle--context-propagation)
  - [Middleware Chain Order](#middleware-chain-order)
  - [Client IP Extraction](#client-ip-extraction-strategy)
  - [Response Handling](#response-handling)
  - [Rate Limiting Algorithm](#rate-limiting-algorithm)
  - [Domain Processing](#domain-processing)
  - [Caching Strategy](#caching-strategy)
  - [Logging Patterns](#logging-patterns)
  - [Database Schema](#database-schema)
  - [Testing Patterns](#testing-patterns)
  - [Performance Optimizations](#performance-optimizations)
  - [Extension Points](#extension-points)
  - [Design Principles](#design-principles)
- [Project Structure](#project-structure)
- [Quick Start](#-quick-start)
- [Configuration](#️-configuration)
- [Testing](#-testing)
- [Performance](#-performance)
- [Monitoring & Logging](#-monitoring--logging)
- [Error Handling](#-error-handling)
- [Security Features](#-security-features)
- [Production Deployment](#-production-deployment)
- [API Rate Limits](#-api-rate-limits)

## 🚀 Features

- **Single & Batch Domain Analysis**: Analyze one or multiple domains in a single request
- **Request Tracing**: Unique X-Request-ID header for every request (correlates with logs)
- **Two-Tier Rate Limiting**: Global and per-IP rate limiting with custom token bucket implementation
- **Pluggable Caching**: Support for memory and Redis caching with configurable TTL
- **Concurrent Processing**: Batch requests processed with controlled concurrency
- **Production Logging**: Structured logging to PostgreSQL with severity levels and request tracking
- **Comprehensive Testing**: ~90% test coverage with unit, integration, and edge case tests
- **Graceful Shutdown**: Proper signal handling and resource cleanup
- **Docker Support**: Complete containerization with Docker Compose
- **Health Checks**: Built-in health endpoints for monitoring

## 📋 API Endpoints

### Single Domain Analysis
```http
GET /api/analyze/{domain}
```

**Example Response:**
```json
{
  "domain": "msn.com",
  "total_advertisers": 189,
  "advertisers": [
    {
      "domain": "google.com",
      "count": 102
    },
    {
      "domain": "appnexus.com", 
      "count": 60
    }
  ],
  "cached": false,
  "timestamp": "2025-12-30T10:30:45Z"
}
```

### Batch Domain Analysis
```http
POST /api/batch-analysis
Content-Type: application/json

{
  "domains": ["msn.com", "cnn.com", "vidazoo.com"]
}
```

**Example Response (HTTP 207 Multi-Status):**
```json
{
  "results": [
    {
      "domain": "msn.com",
      "analysis": { /* ... */ },
      "success": true
    },
    {
      "domain": "invalid-domain.com",
      "error": "domain not found",
      "success": false
    }
  ],
  "summary": {
    "total": 2,
    "succeeded": 1,
    "failed": 1
  },
  "timestamp": "2025-12-30T10:30:45Z"
}
```

### Health Check
```http
GET /health
```

### Response Headers

All API responses include a `X-Request-ID` header for request tracing and debugging:

```http
HTTP/1.1 200 OK
Content-Type: application/json
X-Request-ID: 550e8400-e29b-41d4-a716-446655440000

{
  "domain": "msn.com",
  ...
}
```

The `X-Request-ID` value is a unique UUID assigned to each request. Use this ID to:
- Correlate client requests with server logs
- Track request flow through the system
- Debug issues by querying logs: `SELECT * FROM application_logs WHERE process_id = '<request-id>'`

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Handler  │────│  Analysis Service │────│  AdsTxt Parser  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐    ┌─────────────────┐
         └──────────────│  Rate Limiter   │    │  AdsTxt Fetcher │
                        └─────────────────┘    └─────────────────┘
                                 │                       │
                        ┌─────────────────┐    ┌─────────────────┐
                        │  Cache Service  │    │   HTTP Client   │
                        └─────────────────┘    └─────────────────┘
                                 │
                        ┌─────────────────┐    ┌─────────────────┐
                        │   Memory/Redis  │    │   PostgreSQL    │
                        └─────────────────┘    └─────────────────┘
```

## 🔍 Implementation Details

### Complete Request Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. HTTP Request Arrives                                         │
│    GET /api/analyze/msn.com                                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Logging Middleware (loggingMiddleware)                      │
│    ✓ Extract Client IP (X-Forwarded-For / X-Real-IP / Remote)  │
│    ✓ Create LogEvent with UUID ProcessID                       │
│    ✓ Attach LogEvent to context                                │
│    ✓ Read & buffer request body (restore for later use)        │
│    ✓ Log: http_request_start (method, path, body, params, IP)  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Rate Limiting Middleware (rateLimitingMiddleware)           │
│    ✓ Get LogEvent.ClientIP from context                        │
│    ✓ Check global token bucket (100/sec default)               │
│    ✓ Check per-IP token bucket (10/sec default)                │
│    ✓ If exceeded: Return 429 + X-Request-ID header             │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. CORS Middleware (corsMiddleware)                            │
│    ✓ Add CORS headers (Access-Control-Allow-*)                 │
│    ✓ Handle OPTIONS preflight requests                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Recovery Middleware (recoveryMiddleware)                    │
│    ✓ Defer panic recovery                                      │
│    ✓ If panic: Log error + Return 500 + X-Request-ID           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Handler (AnalyzeSingleDomain)                               │
│    ✓ Extract domain from URL params                            │
│    ✓ Log: domain_analysis (with ProcessID from context)        │
│    ✓ Call AnalysisService.AnalyzeDomain(ctx, domain)           │
│    │                                                            │
│    ├─→ Check Cache (cache_hit / cache_miss logged)             │
│    ├─→ Fetch ads.txt (normalize domain, follow redirects)      │
│    ├─→ Parse ads.txt file                                      │
│    ├─→ Count advertisers                                       │
│    └─→ Cache result                                            │
│                                                                 │
│    ✓ Call writeJSONResponse(w, r, 200, analysis)               │
│       - Sets Content-Type: application/json                    │
│       - Sets X-Request-ID: <ProcessID from context>            │
│       - Writes JSON response                                   │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 7. Logging Middleware Completion                               │
│    ✓ Calculate duration (time.Since(logEvent.StartTime))       │
│    ✓ Capture status code from wrapped ResponseWriter           │
│    ✓ Log: http_request_complete (status, duration, IP)         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 8. HTTP Response Sent to Client                                │
│    HTTP/1.1 200 OK                                              │
│    Content-Type: application/json                              │
│    X-Request-ID: 550e8400-e29b-41d4-a716-446655440000          │
│                                                                 │
│    {"domain": "msn.com", "total_advertisers": 189, ...}        │
└─────────────────────────────────────────────────────────────────┘

All operations use the SAME ProcessID throughout the entire flow.
Query logs: SELECT * FROM application_logs WHERE process_id = '<uuid>';
```

### Request Lifecycle & Context Propagation

The application uses Go's `context.Context` to propagate request metadata through the entire request lifecycle:

1. **LogEvent Creation** (Logging Middleware)
   - Creates a `LogEvent` with unique `ProcessID` (UUID)
   - Extracts client IP from headers (`X-Forwarded-For`, `X-Real-IP`, or `RemoteAddr`)
   - Attaches `LogEvent` to request context: `ctx := logger.WithLogEvent(r.Context(), logEvent)`

2. **Process Types**
   - **`request`**: HTTP requests from clients
   - **`internal`**: Internal server operations (startup, shutdown, background tasks)
   - Each type tracked separately for analytics

3. **Complete Request Flow Logging**
   ```
   http_request_start (logs: method, path, query, url_params, body, client_ip, user_agent)
       ↓
   [Handler processes request - all logs include ProcessID from context]
       ↓
   http_request_complete (logs: method, path, status_code, duration_ms, client_ip)
   ```

4. **Request Body Handling**
   - Body read using `io.ReadAll()` for logging
   - Restored with `io.NopCloser(bytes.NewBuffer())` for downstream handlers
   - Truncated to 1000 chars in logs to prevent bloat
   - Original body remains available for JSON decoding

5. **Context Access in Handlers**
   ```go
   ctx := r.Context()
   logEvent := logger.GetLogEvent(ctx)  // Available everywhere
   h.logger.LogInfo(ctx, "operation", "message", metadata)
   ```

### Middleware Chain Order

**Critical**: Middleware order matters for proper functionality:

```go
1. loggingMiddleware        // Creates LogEvent, logs request start/complete
2. rateLimitingMiddleware   // Requires LogEvent.ClientIP from context
3. corsMiddleware           // Adds CORS headers
4. recoveryMiddleware       // Catches panics, needs context for logging
```

### Client IP Extraction Strategy

Priority order for determining client IP:
1. `X-Forwarded-For` header (first IP in comma-separated list)
2. `X-Real-IP` header
3. `RemoteAddr` (direct connection IP)
4. Falls back to full `RemoteAddr` if port parsing fails

### Response Handling

**Centralized Response Function**:
- All responses use `writeJSONResponse(w, r, statusCode, data)`
- Automatically adds `Content-Type: application/json`
- Automatically adds `X-Request-ID` header from context
- Consistent error handling across all endpoints

**Response Writer Wrapper**:
- Custom `responseWriter` wraps `http.ResponseWriter`
- Captures status code for logging (Go's standard writer doesn't expose this)
- Enables accurate request completion logging

### Rate Limiting Algorithm

**Two-Tier Token Bucket Implementation**:
1. **Global Bucket**: Shared across all requests
   - Refills at configured rate (default: 100/sec)
   - Prevents server overload

2. **Per-IP Buckets**: One bucket per client IP
   - Stored in `sync.Map` for concurrent access
   - Refills at configured rate (default: 10/sec)
   - Prevents single client from monopolizing resources

3. **Bucket Cleanup**: Background goroutine removes stale IP buckets

### Domain Processing

**Domain Normalization**:
- Removes protocol (`http://`, `https://`)
- Removes `www.` prefix
- Removes trailing slashes
- Converts to lowercase
- Example: `HTTPS://WWW.Example.COM/` → `example.com`

**ads.txt Fetching**:
- Follows up to 5 HTTP redirects (301, 302, 307, 308)
- 10-second timeout per fetch
- 1MB file size limit
- TLS verification enabled
- Custom User-Agent header

### Caching Strategy

**Cache Key Generation**:
- Domain used as cache key after normalization
- Example: `msn.com` → cached with all advertiser data

**TTL Management**:
- Configurable via `CACHE_TTL` environment variable
- Default: 3600 seconds (1 hour)
- Expired entries automatically cleaned up (memory cache)
- Redis handles TTL automatically

**Cache Implementations**:
- **Memory**: In-process map with mutex synchronization
- **Redis**: Remote cache with connection pooling

### Concurrent Batch Processing

**Controlled Concurrency**:
- Semaphore pattern limits concurrent fetches
- Default: 10 concurrent domain fetches
- Prevents overwhelming external servers or exhausting connections
- Each domain processed in its own goroutine
- Results collected via channels

### Error Handling Philosophy

**Graceful Degradation**:
- JSON encoding failures logged but don't crash the handler
- Network timeouts return 408 (not 500)
- Partial batch failures return 207 Multi-Status
- All errors include `X-Request-ID` for debugging

**Error Propagation**:
- Errors bubble up through layers
- Each layer adds context
- Final error contains full trace for debugging

### Logging Patterns

**Log Operations** (standardized operation names):
- `http_request_start`, `http_request_complete`
- `domain_analysis`, `batch_analysis`
- `cache_hit`, `cache_miss`, `cache_set`
- `rate_limited`, `panic_recovery`
- Custom operations can be defined per feature

**Log Severity Levels**:
- `low`: Informational (cache hits, successful operations)
- `medium`: Warnings (rate limits, validation errors)
- `high`: Critical errors (panics, database failures)

**Metadata Tracking**:
Every log entry includes:
- `process_id`: Unique UUID (matches X-Request-ID)
- `process_type`: "request" or "internal"
- `timestamp`: UTC timestamp
- `client_ip`: Extracted client IP
- `operation`: Standardized operation name
- `target`: Domain being processed (if applicable)
- `custom_metadata`: Additional context (JSON object)

### Database Schema

**application_logs Table**:
```sql
CREATE TABLE application_logs (
    id UUID PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    severity VARCHAR(10),
    operation VARCHAR(100),
    message TEXT,
    error TEXT,
    target VARCHAR(255),
    process_id UUID,           -- Matches X-Request-ID
    process_type VARCHAR(20),  -- "request" or "internal"
    client_ip VARCHAR(45),
    custom_metadata JSONB
);

-- Index for fast request tracking
CREATE INDEX idx_process_id ON application_logs(process_id);
CREATE INDEX idx_timestamp ON application_logs(timestamp DESC);
```

### Testing Patterns

**Test Organization**:
- Unit tests in `*_test.go` files alongside source
- Integration tests in `integration_test.go`
- Edge cases in `edge_cases_test.go`
- Mocks in `internal/mocks/` and `internal/http/mocks/`

**Mock Strategy**:
- `testify/mock` for interface mocking
- `httptest` for HTTP handler testing
- `miniredis` for Redis cache testing
- Custom test helpers for common scenarios

**Coverage Goals**:
- Critical paths: 100% (parsing, domain analysis)
- HTTP handlers: Comprehensive (all status codes, error paths)
- Middleware: Edge cases (panics, rate limits, cancelled contexts)
- Utilities: High coverage (>90%)

### Performance Optimizations

**Concurrent Processing**:
- Batch requests use goroutines with semaphore
- Channel-based result collection
- Context cancellation propagates to all goroutines

**Memory Management**:
- Request body buffering with size limits
- Cache cleanup routines for expired entries
- Connection pooling for database and Redis

**Network Efficiency**:
- HTTP client reuse (single client instance)
- Keep-alive connections
- Configurable timeouts
- Response body size limits

### Extension Points

**Adding New Endpoints**:
1. Define handler in `internal/http/handlers.go`
2. Register route in `internal/http/router.go`
3. Use `writeJSONResponse()` for consistent response handling
4. Access context: `ctx := r.Context()`
5. Log operations: `h.logger.LogInfo(ctx, "operation_name", ...)`

**Adding New Middleware**:
1. Create middleware function in `internal/http/middlewares.go`
2. Follow signature: `func(http.Handler) http.Handler`
3. Add to chain in `router.go` (mind the order!)
4. Access LogEvent from context if needed: `logger.GetLogEvent(r.Context())`

**Adding New Cache Backend**:
1. Implement `cache.Service` interface
2. Add to `cache/` directory
3. Update `main.go` to support new cache type
4. Add configuration to env vars

**Adding Custom Log Operations**:
1. Define operation constant in `internal/logger/operations.go`
2. Use in handlers: `logger.LogInfo(ctx, logger.OpYourOperation, ...)`
3. Operations are queryable in database for analytics

### Design Principles

**Context-First Architecture**:
- All operations receive `context.Context` as first parameter
- Context carries request metadata (LogEvent, deadlines, cancellation)
- Never store context in structs; pass it explicitly

**Interface-Driven Design**:
- Services defined as interfaces (`cache.Service`, `logger.Service`, etc.)
- Enables mocking for tests
- Allows swapping implementations (memory/Redis cache)

**Fail-Fast Validation**:
- Validate inputs at API boundary (handlers)
- Return clear error messages
- Use appropriate HTTP status codes

**Observability By Default**:
- Every request gets a unique ProcessID
- All operations logged with context
- Metrics can be derived from log queries

### Project Structure

```
.
├── cmd/                          # Application entrypoints
├── internal/
│   ├── cache/                   # Caching layer (memory/Redis)
│   │   ├── memory.go           # In-memory cache implementation
│   │   ├── redis.go            # Redis cache implementation
│   │   └── domainCache/        # Domain-specific cache wrapper
│   ├── config/                  # Configuration management
│   │   └── config.go           # Environment variable loading
│   ├── domainAnalysis/          # Core business logic
│   │   └── analysis.go         # Domain analysis orchestration
│   ├── fetcher/                 # HTTP fetcher for ads.txt
│   │   └── http.go             # HTTP client with retries & timeouts
│   ├── http/                    # HTTP handlers & middleware
│   │   ├── handlers.go         # API endpoint handlers
│   │   ├── middlewares.go      # Logging, CORS, rate limiting
│   │   ├── router.go           # Route registration
│   │   └── server.go           # HTTP server setup
│   ├── logger/                  # Structured logging
│   │   ├── logger.go           # PostgreSQL logger implementation
│   │   └── context.go          # Request context & log events
│   ├── models/                  # Data models
│   │   └── models.go           # Domain models & DTOs
│   ├── parser/                  # ads.txt parser
│   │   └── parser.go           # ads.txt format parsing
│   ├── ratelimit/              # Rate limiting implementation
│   │   └── limiter.go          # Two-tier token bucket limiter
│   └── mocks/                   # Test mocks
├── docker-compose.yml           # Docker orchestration
├── Dockerfile                   # Multi-stage Docker build
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
└── main.go                      # Application entry point
```

## 🚦 Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- PostgreSQL (if not using Docker)

### Using Docker Compose (Recommended)

1. **Clone and start services:**
```bash
git clone <repository-url>
cd Perion_Assignment
docker-compose up -d
```

2. **Test the API:**
```bash
# Health check
curl http://localhost:8080/health

# Analyze single domain  
curl http://localhost:8080/api/analyze/msn.com

# Batch analysis
curl -X POST http://localhost:8080/api/batch-analysis \
  -H "Content-Type: application/json" \
  -d '{"domains": ["msn.com", "cnn.com"]}'
```

### Manual Setup

1. **Set up environment:**
```bash
cp .env.example .env
# Edit .env with your configuration
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Run the application:**
```bash
go run main.go
```

## ⚙️ Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `CACHE_TYPE` | `memory` | Cache backend (`memory` or `redis`) |
| `CACHE_TTL` | `3600` | Cache TTL in seconds |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `DATABASE_URL` | `postgres://...` | PostgreSQL connection URL |
| `GLOBAL_RATE_LIMIT_PER_SEC` | `100` | Global rate limit |
| `PER_IP_RATE_LIMIT_PER_SEC` | `10` | Per-IP rate limit |
| `FETCH_TIMEOUT_SECONDS` | `10` | HTTP fetch timeout |
| `MAX_CONCURRENT_FETCHES` | `10` | Max concurrent domain fetches |

## 🧪 Testing

**Current Test Coverage: ~90% across all packages**

| Package | Coverage |
|---------|----------|
| Cache | 90.3% |
| Fetcher | 95.5% |
| Config | 100% |
| Rate Limiter | 83.6% |
| HTTP Handlers | Comprehensive |
| Domain Analysis | Comprehensive |
| Parser | Comprehensive |

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## 📊 Performance

- **Concurrent Processing**: Batch requests process domains in parallel
- **Rate Limiting**: 
  - Global: 100 requests/second (configurable)
  - Per-IP: 10 requests/second (configurable)
- **Caching**: Configurable TTL (default 1 hour)
- **Timeouts**: 10-second fetch timeout per domain

## 📈 Monitoring & Logging

The application logs all operations to PostgreSQL with structured data:

- **Operations**: domain_analysis, batch_analysis, cache_hit, rate_limited, etc.
- **Severity Levels**: low, medium, high
- **Process ID**: Unique UUID per request (matches X-Request-ID header)
- **Metadata**: Client IP, duration, error details, custom fields

**Example Log Queries:**

```sql
-- Track a specific request using X-Request-ID
SELECT * FROM application_logs
WHERE process_id = '550e8400-e29b-41d4-a716-446655440000'
ORDER BY timestamp;

-- Operation summary for last hour
SELECT operation, severity, COUNT(*)
FROM application_logs
WHERE timestamp >= NOW() - INTERVAL '1 hour'
GROUP BY operation, severity;

-- Failed requests by client IP
SELECT client_ip, COUNT(*) as failures
FROM application_logs
WHERE severity = 'high' AND timestamp >= NOW() - INTERVAL '1 day'
GROUP BY client_ip
ORDER BY failures DESC;
```

## 🚨 Error Handling

The API returns appropriate HTTP status codes with detailed error messages:

| Status Code | Description | Use Case |
|------------|-------------|----------|
| `200 OK` | Success | Successful single domain analysis |
| `207 Multi-Status` | Partial success | Batch analysis with some failures |
| `400 Bad Request` | Invalid input | Invalid request format or domain |
| `404 Not Found` | Resource missing | ads.txt file not found |
| `408 Request Timeout` | Timeout | Fetch timeout exceeded |
| `429 Too Many Requests` | Rate limited | Rate limit exceeded |
| `500 Internal Server Error` | Server error | Unexpected server errors |

**Error Response Format:**
```json
{
  "error": "rate limit exceeded",
  "message": "Please try again later",
  "timestamp": "2025-12-30T10:30:45Z"
}
```

All error responses include the `X-Request-ID` header for tracking and debugging.

## 🔒 Security Features

- **Rate Limiting**: Two-tier protection (global + per-IP)
- **Input Validation**: Domain format validation and sanitization
- **Request Tracing**: Every request tracked with unique UUID (X-Request-ID)
- **Error Handling**: No sensitive information in error responses
- **Security Headers**: CORS, Content-Type, X-Request-ID
- **Structured Logging**: All operations logged with client IP and metadata
- **Non-root Container**: Docker container runs as non-root user
- **Resource Limits**: File size limits, batch size limits, timeout protection

## 🚢 Production Deployment

### Environment-Specific Configs

**Development:**
- Memory cache
- Verbose logging
- Lower rate limits

**Production:**
- Redis cache
- Structured logging
- Higher rate limits
- Load balancer (Nginx)

## 📋 API Rate Limits

- **Global Limit**: 100 requests/second across all clients
- **Per-IP Limit**: 10 requests/second per client IP
- **Batch Size Limit**: Maximum 100 domains per batch request
- **File Size Limit**: Maximum 1MB ads.txt file size

---

**Built with ❤️ using Go, PostgreSQL, Redis, and Docker**