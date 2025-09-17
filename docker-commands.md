# Docker Commands for BTC LTP Service

## Basic Operations

### Start the complete stack
```bash
docker-compose up -d
```

### Stop the complete stack
```bash
docker-compose down
```

### View service status
```bash
docker-compose ps
```

## Development Commands

### Start only Redis (for local development)
```bash
docker-compose up redis -d
```

### Start with Redis Commander (GUI)
```bash
docker-compose --profile tools up -d
```

### Rebuild application after code changes
```bash
docker-compose build btc-ltp-service
docker-compose up -d btc-ltp-service
```

## Logs and Debugging

### View application logs
```bash
docker-compose logs -f btc-ltp-service
```

### View Redis logs
```bash
docker-compose logs -f redis
```

### View all logs
```bash
docker-compose logs -f
```

## Testing Endpoints

### Health check
```bash
curl http://localhost:8080/health
```

### Get BTC price
```bash
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD"
```

### Get multiple pairs
```bash
curl "http://localhost:8080/api/v1/ltp?pair=BTC/USD,ETH/USD,LTC/USD"
```

### Check cached prices
```bash
curl http://localhost:8080/api/v1/ltp/cached
```

## Redis Management

### Access Redis CLI
```bash
docker-compose exec redis redis-cli
```

### Redis Commander Web UI (with --profile tools)
Open: http://localhost:8081

## Environment Configuration

### Custom configuration
```bash
# Create .env file with custom settings
CACHE_BACKEND=redis
REDIS_PASSWORD=mypassword
SUPPORTED_PAIRS=BTC/USD,ETH/USD,LTC/USD,XRP/USD,ADA/USD

# Then start
docker-compose up -d
```

### Production deployment
```bash
# With custom environment
REDIS_PASSWORD=strong-password docker-compose up -d
```

## Monitoring

### Watch container health
```bash
watch "docker-compose ps"
```

### Monitor logs in real-time
```bash
docker-compose logs -f --tail=100
```

## Cleanup

### Remove containers and volumes
```bash
docker-compose down -v
```

### Remove images
```bash
docker-compose down --rmi all
```
