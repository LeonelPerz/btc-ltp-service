# 🔄 Resilience Matrix: WebSocket → REST Fallback

## 📋 Executive Summary

The BTC LTP service implements a robust resilience system that ensures price data availability through an automatic fallback mechanism from **WebSocket to REST**. This document details circuit-breaker thresholds, monitoring metrics, and test scenarios.

## 🏗️ Resilience Architecture

```
┌─────────────────┐    ❌ Failure    ┌─────────────────┐
│   WebSocket     │ ────────────→    │   REST API      │
│   (Primary)     │                  │  (Secondary)    │
└─────────────────┘                  └─────────────────┘
        ↓                                    ↓
   ┌─────────────────────────────────────────────────────────┐
   │            Circuit Breaker Thresholds                  │
   │ • Timeout: 15s                                         │
   │ • Max Retries: 3                                       │
   │ • Request Timeout: 3s                                  │
   └─────────────────────────────────────────────────────────┘
```

## ⚙️ Circuit Breaker: Thresholds and Configuration

### 🔧 Configuration Parameters

| Parameter | Default Value | ENV Variable | Description |
|-----------|---------------|--------------|-------------|
| **FallbackTimeout** | `15s` | `KRAKEN_FALLBACK_TIMEOUT` | Maximum timeout for WebSocket operations |
| **MaxRetries** | `3` | `KRAKEN_MAX_RETRIES` | Maximum number of retries before fallback |
| **RequestTimeout** | `3s` | `KRAKEN_REQUEST_TIMEOUT` | Timeout per individual request |
| **WebSocketURL** | `wss://ws.kraken.com` | `KRAKEN_WEBSOCKET_URL` | Kraken WebSocket URL |
| **RestURL** | `https://api.kraken.com/0/public` | `KRAKEN_REST_URL` | REST API base URL |

### 🎯 Fallback Activation Conditions

The system activates the **WebSocket → REST** fallback when **any** of these conditions are met:

1. **WebSocket Timeout** (`> 15s`)
2. **WebSocket Connection Errors** (connection closed, network unavailable)
3. **Maximum Retries Reached** (`3 failed attempts`)
4. **Panic Recovery** (critical error recovery)
5. **Invalid WebSocket Responses**

## 📊 Resilience Scenarios Matrix

### 🟢 Scenario 1: Normal Operation
| **Condition** | **Behavior** | **Metrics** | **Logs** |
|---------------|-------------|-------------|----------|
| WebSocket available | ✅ Direct response from WS | `btc_ltp_external_api_requests_total{service="kraken",endpoint="websocket"}` | `Successfully retrieved price via WebSocket` |
| Cache available | ✅ Response from cache | `btc_ltp_cache_operations_total{operation="get",result="hit"}` | Cache hit registered |

### 🟡 Scenario 2: Fallback Activated
| **Condition** | **Behavior** | **Metrics** | **Logs** |
|---------------|-------------|-------------|----------|
| WebSocket timeout (>15s) | 🔄 Automatic fallback to REST | `btc_ltp_external_api_retries_total{attempt="1,2,3"}` | `WebSocket failed, falling back to REST API` |
| REST successful | ✅ Response from REST API | `btc_ltp_external_api_requests_total{service="kraken",endpoint="rest"}` | `Successfully retrieved price via REST fallback` |

### 🔴 Scenario 3: Complete Failure
| **Condition** | **Behavior** | **Metrics** | **Logs** |
|---------------|-------------|-------------|----------|
| WebSocket + REST fail | ❌ HTTP 500 error returned | Both error counters incremented | `Both WebSocket and REST failed` |
| Circuit breaker open | 🚫 Requests temporarily blocked | Circuit breaker metrics increment | Detailed error with both failures |

## 🔍 Monitoring Metrics

### 📈 Key Resilience Metrics

```promql
# WebSocket success rate
rate(btc_ltp_external_api_requests_total{service="kraken",endpoint="websocket",status_code="200"}[5m])

# REST fallback rate
rate(btc_ltp_external_api_requests_total{service="kraken",endpoint="rest"}[5m])

# Fallback response time
histogram_quantile(0.95, rate(btc_ltp_external_api_request_duration_seconds_bucket{service="kraken"}[5m]))

# Retries per minute
rate(btc_ltp_external_api_retries_total[1m])
```

### 🎛️ Recommended Dashboards

#### Dashboard 1: General Resilience
```promql
# Panel 1: WebSocket vs REST success rate
sum(rate(btc_ltp_external_api_requests_total{status_code="200"}[5m])) by (endpoint)

# Panel 2: P95 latency by endpoint
histogram_quantile(0.95, sum(rate(btc_ltp_external_api_request_duration_seconds_bucket[5m])) by (le, endpoint))

# Panel 3: Fallback events
increase(btc_ltp_external_api_retries_total[1h])
```

## 🧪 Test Scenarios

### 🔬 Circuit Breaker Tests

#### Test 1: WebSocket Timeout
```bash
# Configure very low timeout to simulate failure
export KRAKEN_FALLBACK_TIMEOUT=100ms
export KRAKEN_MAX_RETRIES=2

# Execute test
curl -X GET "http://localhost:8080/api/v1/price/BTC/USD" \
  -H "Accept: application/json"
```

**Expected result:**
- Logs: `WebSocket timeout after 100ms`
- Automatic fallback to REST
- Retry metrics incremented

#### Test 2: WebSocket Unavailable
```bash
# Configure invalid WebSocket URL
export KRAKEN_WEBSOCKET_URL="wss://invalid.websocket.url"

# Execute test
curl -X GET "http://localhost:8080/api/v1/prices" \
  -H "Accept: application/json"
```

**Expected result:**
- Logs: `Failed to initialize WebSocket connection`
- Immediate REST API usage
- Connection failure metrics

### 📋 Load and Stress Tests

#### Sustained Load Test
```bash
# Use Apache Bench to generate load
ab -n 1000 -c 10 "http://localhost:8080/api/v1/price/BTC/USD"
```

#### Stress Test with Simulated Failures
```bash
# Disconnect WebSocket during high load
# Monitor automatic transition to REST
watch 'curl -s http://localhost:8080/metrics | grep btc_ltp_external_api'
```

## 📝 Log Examples

### ✅ Normal Operation (WebSocket)
```json
{
  "level": "debug",
  "msg": "Successfully retrieved price via WebSocket",
  "pair": "BTC/USD",
  "amount": 45678.90,
  "source": "websocket",
  "timestamp": "2025-09-23T10:15:30Z"
}
```

### 🔄 Fallback Activation
```json
{
  "level": "info",
  "msg": "WebSocket failed, falling back to REST API",
  "pair": "BTC/USD",
  "websocket_error": "timeout: WebSocket timeout after 15s",
  "fallback_timeout": "15s",
  "timestamp": "2025-09-23T10:15:45Z"
}
```

### ✅ Successful REST Fallback
```json
{
  "level": "info",
  "msg": "Successfully retrieved price via REST fallback",
  "pair": "BTC/USD",
  "amount": 45679.15,
  "source": "rest_fallback",
  "rest_duration_ms": 245,
  "timestamp": "2025-09-23T10:15:46Z"
}
```

### ❌ Complete Failure
```json
{
  "level": "error",
  "msg": "Both WebSocket and REST failed",
  "pair": "BTC/USD",
  "websocket_error": "connection closed",
  "rest_error": "HTTP 503: Service Temporarily Unavailable",
  "rest_duration_ms": 5000,
  "timestamp": "2025-09-23T10:15:51Z"
}
```

## 🚨 Recommended Alerts

### 🔔 Critical Alerts

```yaml
groups:
- name: btc-ltp-resilience
  rules:
  # Alert: High fallback rate
  - alert: HighFallbackRate
    expr: rate(btc_ltp_external_api_requests_total{endpoint="rest"}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High WebSocket → REST fallback rate"
      description: "{{$value}} requests/sec are using REST fallback"

  # Alert: WebSocket completely down
  - alert: WebSocketDown
    expr: absent(btc_ltp_external_api_requests_total{endpoint="websocket",status_code="200"})
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "WebSocket completely inactive"
      description: "No successful WebSocket requests registered in 5 minutes"

  # Alert: Both endpoints failing
  - alert: BothEndpointsFailing
    expr: |
      (
        rate(btc_ltp_external_api_requests_total{status_code!="200"}[5m]) /
        rate(btc_ltp_external_api_requests_total[5m])
      ) > 0.5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Massive endpoint failure"
      description: "More than 50% of requests are failing"
```

## 🔧 Configuration for Different Environments

### 🧪 Development (Fast Timeouts)
```yaml
exchange:
  kraken:
    fallback_timeout: 5s    # Fast fallback for development
    max_retries: 2          # Fewer retries
    request_timeout: 2s     # Faster requests
```

### 🏭 Production (Conservative Timeouts)
```yaml
exchange:
  kraken:
    fallback_timeout: 15s   # More time for recovery
    max_retries: 3          # Standard retries
    request_timeout: 3s     # Balanced timeout
```

### 🔍 Testing (Very Fast Timeouts)
```yaml
exchange:
  kraken:
    fallback_timeout: 100ms # For timeout tests
    max_retries: 1          # Immediate fallback
    request_timeout: 50ms   # Ultra-fast requests
```

