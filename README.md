# BTC Last Traded Price Service - Clean Architecture

A production-ready Go microservice that provides real-time Last Traded Price (LTP) data for Bitcoin trading pairs using the Kraken API. Built following **Clean Architecture principles** for maintainability, scalability, and testability.

## 🏗️ Architecture

This service follows **Clean Architecture** with clear separation of concerns:

### Domain Layer (Enterprise Business Rules)
- **Entities**: Core business objects (Price, TradingPair, ConnectionStatus)
- **Value Objects**: Immutable domain concepts (Currency, CacheTTL)
- **Repository Interfaces**: Data access contracts
- **Service Interfaces**: External service contracts

### Application Layer (Application Business Rules)  
- **Use Cases**: Business logic operations (GetLTP, RefreshPrices, GetSupportedPairs)
- **DTOs**: Data transfer objects for application boundaries
- **Application Service**: Orchestrates use cases

### Infrastructure Layer (Frameworks & Drivers)
- **External Services**: Kraken API client with WebSocket/REST hybrid
- **Repository Implementations**: Cache-based data storage
- **Configuration**: Environment-based config management
- **Metrics & Logging**: Observability components

### Interface Layer (Interface Adapters)
- **HTTP Handlers**: REST API endpoints
- **Middleware**: Cross-cutting concerns (CORS, logging, metrics)
- **DTOs**: HTTP-specific data structures

## 🚀 Features

- ⚡ **Real-time data**: WebSocket primary with REST fallback
- 🏗️ **Clean Architecture**: SOLID principles, dependency inversion
- 🔄 **Hybrid connectivity**: Automatic failover between WebSocket and REST
- 🧠 **Smart caching**: Configurable TTL with background refresh
- 📊 **Rich observability**: Prometheus metrics, structured logging
- 🌐 **RESTful API**: Clean JSON API with metadata support
- 🐳 **Production ready**: Docker, graceful shutdown, health checks
- 🔧 **Highly configurable**: Environment-based configuration
- 📈 **Monitoring**: Connection status, rate limiting stats

## 📊 Supported Trading Pairs

The service dynamically loads supported pairs from configuration:
- BTC/USD, BTC/EUR, BTC/CAD (default)
- Configurable via `SUPPORTED_PAIRS` environment variable
- Real-time pair discovery from Kraken API (1200+ pairs available)

## 🔌 API Endpoints

### Core Endpoints

#### Get Last Traded Prices
```bash
GET /api/v1/ltp[?pair=BTC/USD][&include_metadata=true]
```

**Response with Clean Architecture DTOs:**
```json
{
  "ltp": [
    {
      "pair": "BTC/USD",
      "amount": 115482.1,
      "timestamp": "2025-09-15T04:11:22.671156256Z",
      "age": "1.469µs"
    }
  ],
  "metadata": {
    "processing_time": "2.3ms",
    "cache_hits": 1,
    "cache_misses": 0,
    "data_source": "websocket",
    "generated_at": "2025-09-15T04:11:22.671Z"
  }
}
```

#### Get Supported Pairs
```bash
GET /api/v1/pairs[?include_disabled=false]
```

**Response:**
```json
{
  "pairs": ["BTC/USD", "BTC/EUR", "BTC/CAD"],
  "count": 3
}
```

#### Refresh Prices
```bash
POST /api/v1/refresh[?pair=BTC/USD][&force=true]
```

**Response:**
```json
{
  "success": true,
  "refreshed_count": 3,
  "duration": 231790537,
  "message": "Successfully refreshed 3 price(s)"
}
```

#### Connection Status
```bash
GET /api/v1/status[?include_details=true]
```

**Response:**
```json
{
  "status": "healthy",
  "connection_type": "hybrid",
  "last_update": "2025-09-15T04:11:26.156Z",
  "details": {
    "websocket_enabled": true,
    "is_connected": true,
    "fallback_mode": false,
    "reconnect_attempts": 0
  }
}
```

### Monitoring Endpoints

- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics  
- `GET /swagger/` - API documentation

## 🏗️ Project Structure (Clean Architecture)

```
.
├── cmd/
│   └── api/main.go              # Application entry point
├── internal/
│   ├── domain/                  # Enterprise Business Rules
│   │   ├── entity/              # Domain entities
│   │   ├── repository/          # Repository interfaces  
│   │   ├── service/             # Service interfaces
│   │   └── valueobject/         # Value objects
│   ├── application/             # Application Business Rules
│   │   ├── usecase/             # Use cases
│   │   ├── dto/                 # Application DTOs
│   │   └── service/             # Application service
│   ├── infrastructure/          # Frameworks & Drivers
│   │   ├── external/            # External service clients
│   │   ├── repository/          # Repository implementations
│   │   ├── config/              # Configuration
│   │   ├── logger/              # Logging
│   │   └── metrics/             # Metrics
│   └── interface/               # Interface Adapters
│       └── http/                # HTTP interface
│           ├── handler/         # HTTP handlers
│           └── middleware/      # HTTP middleware
└── Docker files, configs, docs...
```

## 🚀 Quick Start

### Using Docker (Recommended)
```bash
# Clone and start
git clone <repository-url>
cd btc-ltp-service
docker-compose up -d

# Test the API
curl http://localhost:8080/api/v1/ltp?pair=BTC/USD
```

### Local Development
```bash
# Install dependencies
go mod download

# Set environment variables
cp config.example.env .env
source .env

# Run the service
go run cmd/api/main.go

# Or build and run
go build -o btc-ltp-service cmd/api/main.go
./btc-ltp-service
```

## ⚙️ Configuration

The service uses environment-based configuration following the 12-factor app methodology:

### Core Configuration
```bash
# Server
SERVER_PORT=8080
LOG_LEVEL=info

# Supported pairs (comma-separated)
SUPPORTED_PAIRS=BTC/USD,BTC/EUR,BTC/CAD

# Cache settings
CACHE_BACKEND=memory  # memory, redis
CACHE_TTL=5m
CACHE_REFRESH_INTERVAL=30s

# Redis (if using redis backend)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

### Kraken API Configuration  
```bash
# WebSocket settings
KRAKEN_WEBSOCKET_ENABLED=true
KRAKEN_WEBSOCKET_URL=wss://ws.kraken.com/
KRAKEN_WEBSOCKET_TIMEOUT=30s
KRAKEN_RECONNECT_DELAY=5s  
KRAKEN_MAX_RECONNECT_TRIES=5

# Rate limiting
KRAKEN_RATE_LIMIT_ENABLED=true
KRAKEN_RATE_LIMIT_CONSERVATIVE=true
KRAKEN_RATE_LIMIT_CAPACITY=10
KRAKEN_RATE_LIMIT_REFILL_RATE=1
KRAKEN_RATE_LIMIT_REFILL_PERIOD=2s
```

## 🔧 Development

### Prerequisites
- Go 1.21+
- Docker & Docker Compose (optional)
- Make (optional)

### Build
```bash
# Development build
go build -o btc-ltp-service cmd/api/main.go

# Production build with optimizations
go build -ldflags="-s -w" -o btc-ltp-service cmd/api/main.go
```

### Testing
```bash
# Run tests
go test ./...

# Run with coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Adding New Use Cases

Thanks to Clean Architecture, adding new functionality is straightforward:

1. **Define entities** in `internal/domain/entity/`
2. **Create use case** in `internal/application/usecase/`  
3. **Add DTOs** in `internal/application/dto/`
4. **Implement handler** in `internal/interface/http/handler/`
5. **Wire up** in `cmd/api/main.go`

## 📊 Monitoring & Observability

### Metrics (Prometheus)
- HTTP request metrics (duration, count, status codes)
- Cache performance (hits, misses, operation duration)
- Kraken API metrics (requests, retries, errors)
- WebSocket connection metrics
- Custom business metrics

### Logging (Structured JSON)
- Request tracing with correlation IDs
- Service events and state changes  
- Error tracking with context
- Performance monitoring

### Health Checks
- Liveness: `/health`
- Readiness: Service dependencies check
- Connection status: `/api/v1/status`

## 🏭 Production Deployment

### Docker Compose
```bash
# Production deployment
docker-compose -f docker-compose.yml up -d

# With Redis cache
docker-compose -f docker-compose.yml -f docker-compose.redis.yml up -d
```

### Environment Variables
Set appropriate values for production:
- `LOG_LEVEL=info`
- `CACHE_BACKEND=redis` (recommended)
- `KRAKEN_RATE_LIMIT_CONSERVATIVE=true`
- Set resource limits in Docker

### Scaling Considerations
- Stateless design enables horizontal scaling
- Redis cache for shared state across instances  
- Load balancer with health check integration
- Consider WebSocket connection limits

## 🔍 Clean Architecture Benefits

This implementation demonstrates:

- **Dependency Inversion**: Infrastructure depends on domain, not vice versa
- **Testability**: Each layer can be tested independently  
- **Flexibility**: Easy to swap implementations (cache, external APIs)
- **Maintainability**: Clear separation of concerns
- **Scalability**: Domain logic isolated from technical concerns

## 🤝 Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Follow Clean Architecture principles
4. Add tests for new use cases
5. Commit changes (`git commit -m 'Add amazing feature'`)
6. Push to branch (`git push origin feature/amazing-feature`)
7. Open Pull Request

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

---

## 🔄 Migration from Legacy

This service was successfully refactored from a monolithic architecture to Clean Architecture:

- **Eliminated 400+ lines** of duplicated/unused code
- **Implemented 4 distinct layers** with clear boundaries  
- **Created 19 new files** following clean architecture
- **Maintained 100% backward compatibility** with existing APIs
- **Added enhanced observability** and monitoring capabilities

The migration demonstrates how to apply Clean Architecture principles to real-world Go services while maintaining production stability.