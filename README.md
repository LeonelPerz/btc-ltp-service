# BTC Last Traded Price Service

A Go microservice that provides real-time Last Traded Price (LTP) data for Bitcoin trading pairs using the Kraken API.

## Supported Trading Pairs

- BTC/USD
- BTC/CHF  
- BTC/EUR

## Features

- ⚡ **Real-time data**: Up-to-the-minute price accuracy with 1-minute caching
- 🚀 **High performance**: In-memory caching with background refresh
- 🌐 **RESTful API**: Clean JSON API with flexible query parameters
- 🐳 **Dockerized**: Ready-to-deploy containerized application
- 🧪 **Tested**: Comprehensive integration tests included
- 📈 **Monitoring**: Health check endpoint and request logging

## API Endpoints

### Get Last Traded Prices
```
GET /api/v1/ltp
```

**Query Parameters:**
- `pair` - Single trading pair (e.g., `pair=BTC/USD`)
- `pairs` - Multiple comma-separated pairs (e.g., `pairs=BTC/USD,BTC/EUR`)
- No parameters - Returns all supported pairs

**Response Format:**
```json
{
  "ltp": [
    {
      "pair": "BTC/CHF",
      "amount": 49000.12
    },
    {
      "pair": "BTC/EUR", 
      "amount": 50000.12
    },
    {
      "pair": "BTC/USD",
      "amount": 52000.12
    }
  ]
}
```

### Health Check
```
GET /health
```

### Supported Pairs
```
GET /api/v1/pairs
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (optional)
- Internet connection (for Kraken API access)

### Option 1: Run with Docker Compose (Recommended)

```bash
# Clone the repository
git clone <repository-url>
cd btc-ltp-service

# Build and start the service
docker-compose up --build

# Service will be available at http://localhost:8080
```

### Option 2: Run with Docker

```bash
# Build the Docker image
docker build -t btc-ltp-service .

# Run the container
docker run -p 8080:8080 btc-ltp-service

# Service will be available at http://localhost:8080
```

### Option 3: Run Locally with Go

```bash
# Clone the repository
git clone https://github.com/LeonelPerz/btc-ltp-service
cd btc-ltp-service

# Download dependencies
go mod tidy

# Run the service
go run cmd/api/main.go

# Or build and run
go build -o btc-ltp-service cmd/api/main.go
./btc-ltp-service
```

## Configuration

The service can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |

## Usage Examples

### Get all supported pairs
```bash
curl http://localhost:8080/api/v1/ltp
```

### Get specific pair
```bash
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"
```

### Get multiple pairs
```bash
curl "http://localhost:8080/api/v1/ltp?pairs=BTC/USD,BTC/EUR"
```

### Health check
```bash
curl http://localhost:8080/health
```

### Get supported pairs list
```bash
curl http://localhost:8080/api/v1/pairs
```

## Testing

### Run All Tests
```bash
go test ./...
```

### Run Integration Tests Only
```bash
go test ./tests/integration/...
```

### Run Tests with Coverage
```bash
go test -cover ./...
```

### Run Benchmarks
```bash
go test -bench=. ./tests/integration/...
```

## Architecture

The service follows a clean architecture pattern:

```
btc-ltp-service/
├── cmd/api/                 # Application entry point
├── internal/
│   ├── handler/            # HTTP handlers and routing
│   ├── service/            # Business logic
│   ├── client/kraken/      # External API client
│   ├── model/              # Data models
│   └── cache/              # Price caching system
├── pkg/utils/              # Shared utilities
├── tests/integration/      # Integration tests
├── Dockerfile              # Container configuration
├── docker-compose.yml      # Multi-container setup
└── README.md               # Documentation
```

## Key Components

- **Handler Layer**: HTTP request handling and routing
- **Service Layer**: Business logic and orchestration
- **Client Layer**: External API integration with Kraken
- **Cache Layer**: In-memory price caching with TTL
- **Models**: Data structures and type definitions

## Performance Features

- **Caching Strategy**: 1-minute cache TTL for up-to-the-minute accuracy
- **Background Refresh**: Automatic cache warming every 30 seconds
- **Concurrent Safe**: Thread-safe cache operations
- **Connection Pooling**: Efficient HTTP client with timeouts

## Monitoring and Observability

- **Health Checks**: Container health monitoring
- **Request Logging**: HTTP request/response logging
- **Error Handling**: Graceful error responses
- **Graceful Shutdown**: Clean service termination

## Development

### Project Structure
```
├── cmd/api/main.go                      # Application entry point
├── internal/handler/ltp.go              # HTTP handlers
├── internal/service/ltp_service.go      # Business logic
├── internal/client/kraken/kraken_client.go # API client
├── internal/model/response.go           # Data models
├── internal/cache/price_cache.go        # Caching system
├── pkg/utils/time_utils.go             # Utilities
└── tests/integration/ltp_test.go        # Integration tests
```

### Adding New Trading Pairs

1. Update `SupportedPairs` map in `internal/model/response.go`
2. Add corresponding Kraken pair mapping
3. Update tests and documentation

### Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Troubleshooting

### Common Issues

**Service fails to start:**
- Check if port 8080 is available
- Verify internet connectivity for Kraken API access
- Check Docker daemon is running (for containerized deployment)

**Empty responses:**
- Kraken API might be temporarily unavailable
- Check service logs for error details
- Verify supported pair names are correct

**Slow responses:**
- Cache might be warming up on first requests
- Network latency to Kraken API
- Check background refresh routine is running

### Logs

View service logs:
```bash
# Docker Compose
docker-compose logs -f

# Docker
docker logs <container-id> -f

# Local
# Logs are printed to stdout
```

## Production Considerations

- Set up proper monitoring and alerting
- Configure load balancing for high availability
- Implement rate limiting if needed
- Set up SSL/TLS termination
- Consider using a reverse proxy (nginx, traefik)
- Monitor Kraken API rate limits

## License

This project is licensed under the MIT License.
