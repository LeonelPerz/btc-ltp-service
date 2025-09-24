# 📈 BTC LTP Service - Bitcoin Last Traded Price API

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-enabled-blue.svg)](https://www.docker.com/)
[![Redis](https://img.shields.io/badge/cache-redis%20%7C%20memory-green.svg)](https://redis.io/)
[![API](https://img.shields.io/badge/API-REST-orange.svg)](http://localhost:8080/docs)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## 🎯 Overview

**BTC LTP Service** is a high-performance, enterprise-grade REST API service that provides real-time Last Traded Price (LTP) data for cryptocurrencies using Kraken Exchange API. Built with **Clean Architecture** principles and **Domain-Driven Design**, it implements a robust WebSocket→REST fallback pattern with intelligent caching and comprehensive observability.

### Key Features

- 🚀 **Real-time Price Feeds** - WebSocket primary, REST API fallback
- 📊 **Multiple Currency Pairs** - BTC/USD, BTC/EUR, BTC/CHF, ETH/USD, and more
- ⚡ **Dual Cache Backend** - Memory or Redis with configurable TTL
- 📈 **Prometheus Metrics** - 30+ metrics for comprehensive monitoring
- 🛡️ **Rate Limiting** - Token bucket algorithm with IP-based throttling
- 📝 **Structured Logging** - JSON format with request tracing
- 🐳 **Docker Ready** - Multi-stage builds with security hardening
- ⚙️ **Clean Architecture** - DDD patterns with clear separation of concerns

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│  Rate Limiter    │───▶│   API Gateway   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
                        ┌────────────────────────────────┼─────────────────────────────────┐
                        │                                ▼                                 │
                   ┌────────────┐  ┌─────────────────────────────────────┐  ┌──────────────┐
                   │ Prometheus │  │           BTC LTP Service           │  │    Logging   │
                   │  Metrics   │  │    (Clean Architecture DDD)        │  │ (Structured) │
                   └────────────┘  └─────────────────────────────────────┘  └──────────────┘
                                                         │
                        ┌────────────────────────────────┼─────────────────────────────────┐
                        │                                ▼                                 │
                ┌───────────────┐  ┌───────────────────────────────────────┐  ┌───────────────┐
                │  Cache Layer  │  │        Fallback Exchange             │  │  Config Mgmt  │
                │ (Memory/Redis)│  │ WebSocket(1°) ←→ REST API(2°)       │  │ (YAML + ENV)  │
                └───────────────┘  └───────────────────────────────────────┘  └───────────────┘
                                                         │
                                             ┌───────────▼──────────┐
                                             │    Kraken Exchange   │
                                             │ (WebSocket + REST)   │
                                             └──────────────────────┘
```

## 🚀 Quick Start

### Docker (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/LeonelPerz/btc-ltp-service
cd btc-ltp-service

# 2. Start the service stack
docker-compose up -d

# 3. Test the API
curl http://localhost:8080/health
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"

# 4. View Swagger documentation
open http://localhost:8080/docs
```

### Local Development

```bash
# 1. Install dependencies
go mod download

# 2. Configure environment
cp env.example .env
export CACHE_BACKEND=memory
export PORT=8080

# 3. Run the service
go run cmd/api/main.go

# 4. With Redis (optional)
docker-compose up redis -d
export CACHE_BACKEND=redis
go run cmd/api/main.go
```

<!-- COVERAGE_START -->
## 🧪 Testing & Coverage

### Test Coverage Report

![Coverage Badge](https://img.shields.io/badge/Coverage-31.1%25-red?style=flat-square&logo=go)

**Current Coverage**: 31.1% overall
- **Cache Package**: 73.3%
- **Kraken Package**: 80.9%
- **Config Package**: 27.1%
- **Exchange Package**: 77.4%

### Test Features Covered

- **Cache eviction mechanisms** - Automatic and manual cleanup of expired entries
- **TTL (Time To Live) validation** - Edge cases including zero, negative, and extreme values
- **Trading pair validation** - Format validation and known pair verification
- **Concurrent cache operations** - Thread-safe operations with race detection
- **Memory cleanup and optimization** - Efficient eviction under memory pressure
- **Error handling and edge cases** - Comprehensive error scenarios
- **Resilience and fallback mechanisms** - WebSocket to REST API fallback

### Running Tests

```bash
# Run all tests with coverage
go test -cover ./...

# Generate detailed coverage report  
./scripts/coverage-report.sh

# View HTML coverage report
open reports/coverage/coverage.html
```

Last updated: 2025-09-24 15:56:00 UTC
<!-- COVERAGE_END -->

# Run specific test suites
go test -v ./internal/infrastructure/repositories/cache/...
go test -v ./internal/infrastructure/exchange/kraken/...
go test -v ./internal/infrastructure/config/...

# Race detection tests
go test -race ./internal/infrastructure/repositories/cache/...
go test -race ./internal/infrastructure/exchange/kraken/...
go test -race ./internal/infrastructure/config/...

### Advanced Test Scenarios

#### Cache Eviction Tests
- TTL-based eviction with different expiration times
- Partial eviction scenarios (some expired, some valid)
- Complete cache cleanup
- Auto-eviction during Set operations
- Concurrent eviction under memory pressure

#### TTL Edge Cases  
- Zero TTL (immediate expiry)
- Negative TTL (pre-expired items)
- Microsecond precision TTL
- Very long TTL (365+ days)
- TTL behavior during concurrent access

#### Trading Pair Validation
- Valid major pairs (BTC/USD, ETH/USD, etc.)
- Case insensitive validation
- Format validation (BASE/QUOTE pattern)
- Unknown pair rejection
- Whitespace and malformed input handling

### Coverage Quality Gates

Our CI/CD pipeline enforces these coverage thresholds:

- 🎯 **Critical Packages**: 70%+ (Cache, Kraken)  


## 📚 API Documentation

### Base URL
- **Local Development**: `http://localhost:8080`
- **Docker**: `http://localhost:8080`

### Authentication

#### API-Key Authentication (Optional)
The service supports optional API-key based authentication that can be enabled via configuration:

**Default State**: Authentication is **disabled** by default
**Toggle Method**: Set `AUTH_ENABLED=true` and `AUTH_API_KEY=your-secret-key` environment variables

**Authenticated Endpoints**: All `/api/v1/*` endpoints when auth is enabled
**Always Unauthenticated**: `/health`, `/ready`, `/metrics`, `/swagger/`, `/docs`

**Usage Example**:
```bash
# Enable authentication
export AUTH_ENABLED=true
export AUTH_API_KEY=mi-clave-secreta-2024

# Make authenticated requests
curl -H "X-API-Key: mi-clave-secreta-2024" "http://localhost:8080/api/v1/ltp?pair=BTC/USD"

# These endpoints work without authentication even when auth is enabled
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

**Configuration**:
```yaml
auth:
  enabled: false               # Toggle authentication
  api_key: ""                 # Your API key (set via environment)
  header_name: "X-API-Key"    # Header name for API key
  unauth_paths:               # Paths exempt from authentication
    - "/health"
    - "/ready"  
    - "/metrics"
    - "/swagger/"
    - "/docs"
```

---

## 🔌 API Endpoints

### 📊 Price Operations

#### Get Last Traded Prices
```http
GET /api/v1/ltp?pair={pairs}
```

**Description**: Retrieves the latest traded prices for specified cryptocurrency pairs.

**Query Parameters**:
- `pair` (optional): Comma-separated list of trading pairs (e.g., `BTC/USD,ETH/USD`)
- If empty, returns all supported pairs

**Response** (200 OK):
```json
{
  "ltp": [
    {
      "pair": "BTC/USD",
      "amount": 50123.45
    },
    {
      "pair": "ETH/USD", 
      "amount": 3456.78
    }
  ]
}
```

**Partial Success** (206 Partial Content):
```json
{
  "ltp": [
    {
      "pair": "BTC/USD",
      "amount": 50123.45
    }
  ],
  "errors": [
    {
      "pair": "INVALID/PAIR",
      "error": "Failed to fetch price",
      "code": "PRICE_FETCH_ERROR",
      "message": "Unsupported trading pair"
    }
  ]
}
```

**Examples**:
```bash
# All supported pairs
curl "http://localhost:8080/api/v1/ltp"

# Single pair
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"

# Multiple pairs
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD,ETH/USD,BTC/EUR"
```

---

#### Refresh Prices (Admin)
```http
POST /api/v1/ltp/refresh?pairs={pairs}
```

**Description**: Manually refreshes cached prices for specified pairs.

**Query Parameters**:
- `pairs` (required): Comma-separated list of trading pairs

**Response** (200 OK):
```json
{
  "message": "Prices refreshed successfully",
  "pairs": ["BTC/USD", "ETH/USD"]
}
```

**Example**:
```bash
curl -X POST "http://localhost:8080/api/v1/ltp/refresh?pairs=BTC/USD,ETH/USD"
```

---

#### Get Cached Prices (Debug)
```http
GET /api/v1/ltp/cached
```

**Description**: Returns all prices currently stored in cache (for debugging/monitoring).

**Response** (200 OK):
```json
{
  "ltp": [
    {
      "pair": "BTC/USD",
      "amount": 50123.45
    }
  ]
}
```

---

### 🏥 Health & Monitoring

#### Health Check
```http
GET /health
```

**Description**: Basic service health check.

**Response** (200 OK):
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "services": {
    "service": "running"
  }
}
```

---

#### Readiness Check
```http
GET /ready
```

**Description**: Readiness probe that validates dependencies (cache, external APIs).

**Response** (200 OK):
```json
{
  "status": "ready",
  "timestamp": "2024-01-01T12:00:00Z", 
  "services": {
    "cache": "ready",
    "service": "ready"
  }
}
```

**Response** (503 Service Unavailable):
```json
{
  "status": "unhealthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "services": {
    "cache": "error: connection refused",
    "service": "ready"
  }
}
```

---

#### Prometheus Metrics
```http
GET /metrics
```

**Description**: Prometheus-compatible metrics endpoint.

**Content-Type**: `text/plain`

---

## 📊 Supported Trading Pairs

The service supports the following cryptocurrency pairs by default:

| Pair | Description |
|------|-------------|
| **BTC/USD** | Bitcoin to US Dollar |
| **BTC/EUR** | Bitcoin to Euro |
| **BTC/CHF** | Bitcoin to Swiss Franc |
| **ETH/USD** | Ethereum to US Dollar |
| **LTC/USD** | Litecoin to US Dollar |
| **XRP/USD** | Ripple to US Dollar |

**Configurable**: Additional pairs can be configured via the `SUPPORTED_PAIRS` environment variable.

---

## ⚙️ Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| **SERVER** | | |
| `PORT` | `8080` | HTTP server port |
| `SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| **CACHE** | | |
| `CACHE_BACKEND` | `memory` | Cache backend: `memory` or `redis` |
| `CACHE_TTL` | `30s` | Cache TTL duration |
| `REDIS_ADDR` | `localhost:6379` | Redis server address |
| `REDIS_PASSWORD` | | Redis password (if required) |
| `REDIS_DB` | `0` | Redis database number |
| **BUSINESS** | | |
| `SUPPORTED_PAIRS` | `BTC/USD,ETH/USD,LTC/USD,XRP/USD` | Supported trading pairs |
| **RATE LIMITING** | | |
| `RATE_LIMIT_ENABLED` | `true` | Enable/disable rate limiting |
| `RATE_LIMIT_CAPACITY` | `100` | Requests per bucket |
| `RATE_LIMIT_REFILL_RATE` | `10` | Refill rate per second |
| **LOGGING** | | |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| **KRAKEN API** | | |
| `KRAKEN_TIMEOUT` | `10s` | HTTP client timeout |
| `KRAKEN_REQUEST_TIMEOUT` | `3s` | Per-request timeout |
| `KRAKEN_FALLBACK_TIMEOUT` | `15s` | WebSocket timeout |
| `KRAKEN_MAX_RETRIES` | `3` | Retry attempts |

### Configuration Files & Precedence System

The service implements a **robust hierarchical configuration system** with fail-fast validation:

#### 🏗️ Configuration Precedence Order
```
1. Default Values (in code) 
    ↓ (overridden by)
2. Base config.yaml  
    ↓ (overridden by)
3. Environment-specific config.{env}.yaml
    ↓ (overridden by) 
4. Environment Variables (highest priority)
```

#### 📁 Available Configuration Files

| File | Purpose | Description |
|------|---------|-------------|
| **Base Configuration** | | |
| `config.yaml` | Production default | Base configuration for all environments |
| `config.production.yaml` | Production env | Optimized settings for production deployment |
| `config.demo-precedence.yaml` | Precedence demo | Demonstrates ENV override behavior |
| **Testing & Validation** | | |
| `config.test-bad-ttl.yaml` | TTL validation | Contains invalid TTL values for testing |
| `config.test-invalid-types.yaml` | Type validation | Invalid data types for parsing tests |
| `config.test-zero-values.yaml` | Zero value detection | Explicit zero values that should fail |
| `config.explicit-zero.yaml` | Explicit zeros | Zero values with units (0s, 0ms) |
| `config.test-missing-units.yaml` | Unit validation | Numbers without time units |
| **Environment Files** | | |
| `demo.env` | ENV variables demo | Sample environment variable overrides |

#### 🎯 Precedence Examples

**Base YAML Configuration**:
```yaml
# configs/config.demo-precedence.yaml
server:
  port: 9000              # Will be overridden by ENV
cache:
  backend: redis          # Will be overridden by ENV
  ttl: 60s               # Will be overridden by ENV
logging:
  level: debug           # Will be overridden by ENV
```

**Environment Override**:
```bash
# These ENV vars override YAML values
export PORT=8080                    # Overrides port: 9000
export CACHE_BACKEND=memory         # Overrides backend: redis  
export CACHE_TTL=30s               # Overrides ttl: 60s
export LOG_LEVEL=info              # Overrides level: debug

go run cmd/api/main.go -config configs/config.demo-precedence.yaml
# Result: Uses port=8080, backend=memory, ttl=30s, level=info
```

#### ⚡ Fail-Fast Configuration Validation

The service implements **comprehensive validation** that fails fast on startup:

**Bad TTL Detection**:
```bash
# ❌ These will fail immediately on startup:
CACHE_TTL=50ms go run cmd/api/main.go
# Error: "TTL too short: 50ms, minimum 100ms (causes excessive cache churn)"

CACHE_TTL=25h go run cmd/api/main.go  
# Error: "TTL too long: 25h, maximum 24h (stale data risk)"
```

**Unknown Trading Pairs Detection**:
```bash
# ❌ This will fail immediately:
SUPPORTED_PAIRS="BTC/USD,DOGE/MOON" go run cmd/api/main.go
# Error: "unknown trading pairs: [DOGE/MOON], supported pairs: [BTC/USD, ETH/USD, ...]"
```

**Environment Detection**: Automatically detects environment via `ENVIRONMENT` variable or falls back to `development`.

#### 🚀 Quick Configuration Examples

```bash
# 1. Use default configuration
go run cmd/api/main.go

# 2. Use specific config file
go run cmd/api/main.go -config configs/config.production.yaml

# 3. Override with environment variables
PORT=9000 CACHE_TTL=60s go run cmd/api/main.go

# 4. Load from environment file
source configs/demo.env && go run cmd/api/main.go

# 5. Test fail-fast validation
go run cmd/api/main.go -config configs/config.test-bad-ttl.yaml
```

#### 🧪 Configuration Testing & Validation Scripts

The service includes automated scripts for testing configuration validation:

```bash
# Test all configuration validation scenarios
./scripts/demo-config-validation.sh

# Test invalid data types handling  
./scripts/demo-invalid-types.sh

# Manual testing examples:
CACHE_TTL=10ms go run cmd/api/main.go          # ❌ TTL too short
SUPPORTED_PAIRS="FAKE/COIN" go run cmd/api/main.go  # ❌ Unknown pair
PORT=8080 LOG_LEVEL=debug go run cmd/api/main.go    # ✅ Valid override
```

#### 🔍 Configuration Validation Features

| Validation Type | Examples | Error Behavior |
|----------------|----------|----------------|
| **TTL Validation** | `50ms`, `25h`, `invalid` | Fails before service start |
| **Trading Pairs** | `DOGE/MOON`, `INVALID` | Lists valid alternatives |
| **Data Types** | `"abc"` for numbers | Viper parsing error |
| **Missing Units** | `30` instead of `30s` | Duration parsing error |
| **Zero Values** | `ttl: 0`, `port: 0` | Detected as parsing errors |

**Key Benefits**:
- 🚀 **Fail-Fast**: Invalid configs prevent service startup
- 🔍 **Specific Errors**: Clear error messages with suggestions  
- 🛡️ **Multiple Layers**: Viper parsing + Custom validation + Business rules
- 📝 **Helpful Messages**: Shows valid alternatives and reasoning

For complete validation examples and testing, see [`docs/CONFIG-VALIDATION.md`](docs/CONFIG-VALIDATION.md).

---

## 📈 Monitoring & Observability

### Prometheus Metrics

The service exposes 30+ metrics across different categories:

#### HTTP Metrics
- `btc_ltp_http_requests_total` - Total HTTP requests by method/path/status
- `btc_ltp_http_request_duration_seconds` - Request duration histogram
- `btc_ltp_http_request_size_bytes` - Request size histogram
- `btc_ltp_http_response_size_bytes` - Response size histogram

#### Cache Metrics
- `btc_ltp_cache_operations_total` - Cache operations counter (hit/miss/error)
- `btc_ltp_cache_keys` - Number of keys in cache
- `btc_ltp_cache_hits_total` - Cache hits counter
- `btc_ltp_cache_misses_total` - Cache misses counter

#### External API Metrics
- `btc_ltp_external_api_requests_total` - External API requests
- `btc_ltp_external_api_request_duration_seconds` - External API latency
- `btc_ltp_external_api_retries_total` - Retry attempts

#### Business Metrics
- `btc_ltp_price_requests_total` - Requests per trading pair
- `btc_ltp_current_prices` - Current prices gauge
- `btc_ltp_price_age_seconds` - Price age in cache

#### Rate Limiting Metrics
- `btc_ltp_rate_limit_requests_total` - Rate limit decisions
- `btc_ltp_rate_limit_tokens_remaining` - Remaining tokens per client

### Structured Logging

All logs are structured in JSON format with contextual information:

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "INFO",
  "message": "HTTP request processed",
  "request_id": "req-abc123",
  "service": "btc-ltp-service",
  "version": "1.0.0",
  "fields": {
    "http_method": "GET",
    "http_path": "/api/v1/ltp",
    "http_status_code": 200,
    "duration_ms": 45.2,
    "cache_hit": true,
    "pairs_count": 3
  }
}
```

---

## 🚀 Performance & Benchmarks

### Load Testing

The service includes comprehensive benchmarking scripts:

```bash
# Run all benchmarks
./benchmarks/scripts/run_benchmarks.sh

# Cache effectiveness test
BASE_URL=http://localhost:8080 k6 run benchmarks/k6/cache_effectiveness.js

# Load testing
BASE_URL=http://localhost:8080 k6 run benchmarks/k6/load_test.js

# Stress testing
BASE_URL=http://localhost:8080 k6 run benchmarks/k6/stress_test.js
```
[`benchmarks/benchmark-analysis.md`](benchmarks/benchmark-analysis.md). 

### Expected Performance

| Metric | Target | Description |
|--------|--------|-------------|
| **Cache Hit Rate** | 85%+ | Under normal load |
| **Response Time** | <50ms | With cache hit |
| **Throughput** | 300+ RPS | Concurrent users |
| **Availability** | 99.9% | Service uptime |

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# With race detection
go test -race ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Manual Testing

```bash
# Basic functionality
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"

# Multiple pairs
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD,ETH/USD"

# Health checks
curl http://localhost:8080/health
curl http://localhost:8080/ready

# Metrics
curl http://localhost:8080/metrics

# Cache inspection
curl http://localhost:8080/api/v1/ltp/cached
```

---

## 🛡️ Security

### Security Features

- **Rate Limiting**: Token bucket algorithm prevents abuse
- **Input Validation**: Comprehensive request validation
- **Docker Security**: Non-root user, minimal attack surface
- **Error Handling**: No sensitive information leakage
- **CORS**: Configurable cross-origin policies

### Security Best Practices

- Run with non-root user in production
- Use HTTPS in production environments  
- Configure rate limiting based on expected load
- Monitor metrics for anomalous patterns
- Keep dependencies updated regularly

---

## 📦 Deployment

### Docker Deployment

```yaml
version: '3.8'
services:
  btc-ltp-service:
    build: .
    ports:
      - "8080:8080"
    environment:
      - CACHE_BACKEND=redis
      - REDIS_ADDR=redis:6379
    depends_on:
      - redis
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

### Production Considerations

- **Environment Variables**: Use secrets management for sensitive data
- **Resource Limits**: Set appropriate CPU/memory limits
- **Health Checks**: Configure container health checks
- **Logging**: Centralize log aggregation
- **Monitoring**: Set up Prometheus + Grafana dashboards
- **Alerting**: Configure alerts for critical metrics

---

## 🔧 Development

### Project Structure

```
btc-ltp-service/
├── cmd/
│   └── api/                    # Application entry point
├── internal/
│   ├── application/            # Application layer (DTOs, services)
│   ├── domain/                 # Domain layer (entities, interfaces)
│   └── infrastructure/         # Infrastructure layer (external concerns)
│       ├── config/             # Configuration management
│       ├── exchange/           # Exchange clients (Kraken)
│       ├── logging/            # Structured logging
│       ├── metrics/            # Prometheus metrics
│       ├── repositories/       # Data access (cache)
│       └── web/                # HTTP layer (handlers, middleware)
├── configs/                    # Configuration files & validation examples
│   ├── config.yaml             # Base configuration
│   ├── config.production.yaml  # Production environment 
│   ├── config.demo-precedence.yaml # Precedence demonstration
│   ├── config.test-*.yaml      # Validation test cases
│   └── demo.env               # Environment variables example
├── scripts/                    # Configuration demo & validation scripts
│   ├── demo-config-validation.sh # Comprehensive validation demo
│   └── demo-invalid-types.sh   # Invalid data types demo
├── benchmarks/                 # Load testing scripts
├── docs/                       # Documentation
│   └── CONFIG-VALIDATION.md    # Configuration validation guide
└── docker-compose.yml          # Development stack
```

---

## 📋 Error Codes

| Code | Description | HTTP Status |
|------|-------------|-------------|
| `INVALID_PARAMETER` | Invalid request parameters | 400 |
| `UNSUPPORTED_PAIR` | Trading pair not supported | 400 |
| `PRICE_FETCH_ERROR` | Failed to fetch price data | 500 |
| `CACHE_ERROR` | Cache operation failed | 500 |
| `ALL_PRICES_FAILED` | All price requests failed | 500 |
| `RATE_LIMIT_EXCEEDED` | Rate limit exceeded | 429 |
| `ENCODING_ERROR` | Response encoding failed | 500 |

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🔗 Links

- [Swagger/OpenAPI Documentation](http://localhost:8080/docs) (when running)
- [Prometheus Metrics](http://localhost:8080/metrics) (when running)
- [Kraken API Documentation](https://docs.kraken.com/rest/)
- [Go Documentation](https://golang.org/doc/)
------

