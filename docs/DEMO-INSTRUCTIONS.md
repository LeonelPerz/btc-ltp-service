# 🚀 Resilience Matrix Demo Instructions

## 📋 Overview

This guide will help you run a complete demonstration of the WebSocket → REST resilience system with circuit-breaker, including detailed logs and real-time metrics.

## 🎯 Demo Objectives

Demonstrate:
- ✅ Automatic WebSocket to REST fallback
- ✅ Circuit-breaker with configurable thresholds
- ✅ Real-time Prometheus metrics
- ✅ Structured logging of failover events
- ✅ Behavior under different failure scenarios

## 🛠️ Environment Setup

### Option 1: Quick Demo (Forced Configuration)

```bash
# 1. Use configuration that forces fallback
export CONFIG_FILE=configs/config.demo-resilience.yaml

# 2. Start the service
go run cmd/api/main.go

# 3. In another terminal, run the demo
./scripts/demo-resilience.sh
```

### Option 2: Complete Demo (With Docker)

```bash
# 1. Build and start with docker-compose
docker-compose up --build -d

# 2. Run the demo
./scripts/demo-resilience.sh

# 3. View logs in real-time
docker-compose logs -f btc-ltp-service
```

### Option 3: Demo with Environment Variables

```bash
# 1. Configure aggressive timeouts for demo
export KRAKEN_FALLBACK_TIMEOUT=100ms
export KRAKEN_MAX_RETRIES=2  
export KRAKEN_WEBSOCKET_URL=wss://invalid-demo-url.com
export LOG_LEVEL=debug

# 2. Start service
go run cmd/api/main.go

# 3. Run demo
./scripts/demo-resilience.sh
```

## 🎬 Demo Execution

### Complete Demo Script

```bash
# Complete demo with all scenarios
./scripts/demo-resilience.sh run

# Only show current metrics
./scripts/demo-resilience.sh metrics

# Only create demo configuration
./scripts/demo-resilience.sh config

# Show useful Prometheus queries
./scripts/demo-resilience.sh queries
```

### Manual Commands for Step-by-Step Demo

```bash
# 1. Check initial status
curl http://localhost:8080/health

# 2. Request individual price (will activate fallback)
curl http://localhost:8080/api/v1/price/BTC/USD

# 3. Request multiple prices
curl "http://localhost:8080/api/v1/prices?pairs=BTC/USD,ETH/USD,LTC/USD"

# 4. View resilience metrics
curl http://localhost:8080/metrics | grep -E "(fallback|websocket)"

# 5. Repeat requests to see consistency
for i in {1..10}; do
  echo "Request $i:"
  curl -w "Time: %{time_total}s\n" -s http://localhost:8080/api/v1/price/BTC/USD | jq .
  sleep 1
done
```

## 📊 Metrics to Observe

### Key Resilience Metrics

```promql
# Fallback activations by reason
btc_ltp_fallback_activations_total

# Fallback operations duration
btc_ltp_fallback_duration_seconds

# WebSocket connection status (1=connected, 0=disconnected)
btc_ltp_websocket_connection_status

# WebSocket reconnection attempts
btc_ltp_websocket_reconnection_attempts_total

# Requests by endpoint (websocket vs rest)
btc_ltp_external_api_requests_total
```

### Suggested Dashboards

```promql
# Panel 1: Fallback Rate
rate(btc_ltp_fallback_activations_total[1m])

# Panel 2: Latency by Endpoint
histogram_quantile(0.95, rate(btc_ltp_external_api_request_duration_seconds_bucket[5m]))

# Panel 3: System Health Status
btc_ltp_websocket_connection_status
```

## 📝 Expected Logs

### Startup Log (WebSocket Failure)
```json
{
  "level": "warn",
  "msg": "Failed to initialize WebSocket connection at startup", 
  "error": "connection refused",
  "websocket_url": "wss://invalid-demo-websocket-url.com",
  "fallback_timeout": "200ms"
}
```

### Fallback Activated Log
```json
{
  "level": "info", 
  "msg": "WebSocket failed, falling back to REST API",
  "pair": "BTC/USD",
  "websocket_error": "connection refused", 
  "fallback_reason": "connection_error",
  "fallback_timeout": "200ms"
}
```

### Successful Fallback Log
```json
{
  "level": "info",
  "msg": "Successfully retrieved price via REST fallback",
  "pair": "BTC/USD", 
  "amount": 43250.50,
  "source": "rest_fallback",
  "rest_duration_ms": 245,
  "fallback_duration_ms": 267
}
```

## 🔍 Test Scenarios

### Scenario 1: WebSocket Timeout
```bash
# Configure very short timeout
export KRAKEN_FALLBACK_TIMEOUT=50ms

# Expected result: Fast fallback to REST
```

### Scenario 2: Invalid WebSocket URL
```bash
# Configure invalid URL
export KRAKEN_WEBSOCKET_URL=wss://invalid-url.com

# Expected result: Immediate fallback to REST
```

### Scenario 3: Maximum Retries
```bash  
# Configure few retries
export KRAKEN_MAX_RETRIES=1

# Expected result: Fallback after 1 retry
```

## 📈 Results Interpretation

### ✅ Expected Behavior

1. **First Request**: May take longer (200-500ms) due to WebSocket attempt + fallback
2. **Subsequent Requests**: Faster (~100-300ms) using REST directly
3. **Metrics**: 
   - `fallback_activations_total` increments with each request
   - `websocket_connection_status` remains at 0
   - `external_api_requests_total{endpoint="rest"}` increments

### 🚨 Problem Indicators

- Timeouts > 5 seconds → Check network configuration
- Continuous 500 errors → Verify Kraken API connectivity
- No metrics → Check /metrics endpoint

## 🛠️ Troubleshooting

### Problem: Service doesn't start
```bash
# Check available port
netstat -tlnp | grep :8080

# Check startup logs
go run cmd/api/main.go 2>&1 | head -20
```

### Problem: No fallback metrics
```bash
# Check metrics endpoint
curl http://localhost:8080/metrics | grep btc_ltp

# Check logging configuration
export LOG_LEVEL=debug
```

### Problem: Fallback doesn't activate
```bash
# Force invalid WebSocket URL
export KRAKEN_WEBSOCKET_URL=wss://invalid.com

# Check in logs
docker-compose logs btc-ltp-service | grep -i fallback
```

The system maintains high availability even when WebSocket fails completely! 🚀
