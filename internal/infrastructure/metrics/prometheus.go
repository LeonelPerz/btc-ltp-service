package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for the BTC LTP Service
var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets, // Standard buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
		},
		[]string{"method", "path"},
	)

	HTTPRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_http_request_size_bytes",
			Help:    "HTTP request size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)

	HTTPResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "path"},
	)

	// Cache Metrics
	CacheOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_cache_operations_total",
			Help: "Total number of cache operations",
		},
		[]string{"operation", "result"}, // operation: get/set/delete, result: hit/miss/success/error
	)

	CacheKeys = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_cache_keys",
			Help: "Number of keys currently in cache",
		},
		[]string{"cache_type"}, // cache_type: memory/redis
	)

	// External API Metrics
	ExternalAPIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_external_api_requests_total",
			Help: "Total number of external API requests",
		},
		[]string{"service", "endpoint", "status_code"},
	)

	ExternalAPIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_external_api_request_duration_seconds",
			Help:    "External API request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0}, // External APIs can be slower
		},
		[]string{"service", "endpoint"},
	)

	ExternalAPIRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_external_api_retries_total",
			Help: "Total number of external API retry attempts",
		},
		[]string{"service", "endpoint", "attempt"},
	)

	// Business Metrics
	PriceRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_price_requests_total",
			Help: "Total number of price requests by trading pair",
		},
		[]string{"pair", "cache_result"}, // cache_result: hit/miss
	)

	PriceRefreshesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_price_refreshes_total",
			Help: "Total number of price refresh operations",
		},
		[]string{"result"}, // result: success/error
	)

	CurrentPrices = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_current_prices",
			Help: "Current cryptocurrency prices",
		},
		[]string{"pair"},
	)

	PriceAge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_price_age_seconds",
			Help: "Age of cached prices in seconds",
		},
		[]string{"pair"},
	)

	// Rate Limiting Metrics
	RateLimitRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_rate_limit_requests_total",
			Help: "Total number of requests processed by rate limiter",
		},
		[]string{"result"}, // result: allowed/blocked
	)

	RateLimitTokensRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_rate_limit_tokens_remaining",
			Help: "Number of tokens remaining in rate limiter buckets",
		},
		[]string{"client_id"}, // client_id: IP address or identifier
	)

	// Kraken Rate Limiting Metrics
	KrakenRateLimitDrops = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_kraken_rate_limit_drops_total",
			Help: "Number of requests dropped due to Kraken rate limiting (429 responses)",
		},
		[]string{"endpoint"},
	)

	KrakenBackoffDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_kraken_backoff_duration_seconds",
			Help:    "Duration of backoff delays due to Kraken rate limiting",
			Buckets: []float64{0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0},
		},
		[]string{"endpoint", "attempt"},
	)

	// Application Metrics
	ApplicationInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_application_info",
			Help: "Application information",
		},
		[]string{"version", "build_time", "go_version"},
	)

	UptimeSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "btc_ltp_uptime_seconds",
			Help: "Application uptime in seconds",
		},
	)

	// WebSocket Metrics
	WebSocketChannelDrops = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_ws_channel_drops_total",
			Help: "Total de actualizaciones de precio descartadas por canal lleno",
		},
		[]string{"pair"},
	)

	// Resilience and Fallback Metrics
	FallbackActivationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_fallback_activations_total",
			Help: "Total number of fallback activations from WebSocket to REST",
		},
		[]string{"reason", "pair"}, // reason: timeout/connection_error/max_retries/panic
	)

	FallbackDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_fallback_duration_seconds",
			Help:    "Duration of fallback operations from WebSocket failure to REST success",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.0, 5.0, 10.0, 15.0},
		},
		[]string{"pair"},
	)

	WebSocketConnectionStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_websocket_connection_status",
			Help: "WebSocket connection status (1=connected, 0=disconnected)",
		},
		[]string{},
	)

	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half_open)",
		},
		[]string{"service", "endpoint"},
	)

	WebSocketReconnectionAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_websocket_reconnection_attempts_total",
			Help: "Total number of WebSocket reconnection attempts",
		},
		[]string{"reason"}, // reason: startup/connection_lost/manual
	)
)

// Helper functions for common metric operations

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path string, statusCode int, duration float64, requestSize, responseSize int64) {
	HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)

	if requestSize > 0 {
		HTTPRequestSizeBytes.WithLabelValues(method, path).Observe(float64(requestSize))
	}
	if responseSize > 0 {
		HTTPResponseSizeBytes.WithLabelValues(method, path).Observe(float64(responseSize))
	}
}

// RecordCacheOperation records cache operation metrics
func RecordCacheOperation(operation, result string) {
	CacheOperationsTotal.WithLabelValues(operation, result).Inc()
}

// RecordExternalAPICall records external API call metrics
func RecordExternalAPICall(service, endpoint string, statusCode int, duration float64) {
	ExternalAPIRequestsTotal.WithLabelValues(service, endpoint, strconv.Itoa(statusCode)).Inc()
	ExternalAPIRequestDuration.WithLabelValues(service, endpoint).Observe(duration)
}

// RecordExternalAPIRetry records external API retry attempts
func RecordExternalAPIRetry(service, endpoint string, attempt int) {
	ExternalAPIRetries.WithLabelValues(service, endpoint, strconv.Itoa(attempt)).Inc()
}

// RecordWebSocketChannelDrop incrementa contador de descartes por canal lleno
func RecordWebSocketChannelDrop(pair string) {
	WebSocketChannelDrops.WithLabelValues(pair).Inc()
}

// RecordPriceRequest records price request metrics
func RecordPriceRequest(pair string, cacheHit bool) {
	cacheResult := "miss"
	if cacheHit {
		cacheResult = "hit"
	}
	PriceRequestsTotal.WithLabelValues(pair, cacheResult).Inc()
}

// UpdateCurrentPrice updates current price gauge
func UpdateCurrentPrice(pair string, price float64) {
	CurrentPrices.WithLabelValues(pair).Set(price)
}

// UpdatePriceAge updates price age gauge
func UpdatePriceAge(pair string, ageSeconds float64) {
	PriceAge.WithLabelValues(pair).Set(ageSeconds)
}

// RecordRateLimitResult records rate limiting results
func RecordRateLimitResult(allowed bool) {
	result := "blocked"
	if allowed {
		result = "allowed"
	}
	RateLimitRequestsTotal.WithLabelValues(result).Inc()
}

// UpdateRateLimitTokens updates remaining tokens gauge
func UpdateRateLimitTokens(clientID string, tokens float64) {
	RateLimitTokensRemaining.WithLabelValues(clientID).Set(tokens)
}

// RecordKrakenRateLimitDrop records requests dropped due to Kraken 429 responses
func RecordKrakenRateLimitDrop(endpoint string) {
	KrakenRateLimitDrops.WithLabelValues(endpoint).Inc()
}

// RecordKrakenBackoffDuration records duration of backoff delays
func RecordKrakenBackoffDuration(endpoint string, attempt int, duration float64) {
	KrakenBackoffDuration.WithLabelValues(endpoint, strconv.Itoa(attempt)).Observe(duration)
}

// SetApplicationInfo sets application information
func SetApplicationInfo(version, buildTime, goVersion string) {
	ApplicationInfo.WithLabelValues(version, buildTime, goVersion).Set(1)
}

// UpdateUptime updates application uptime
func UpdateUptime(seconds float64) {
	UptimeSeconds.Set(seconds)
}

// Resilience and Fallback Metrics Functions

// RecordFallbackActivation records when fallback from WebSocket to REST is activated
func RecordFallbackActivation(reason, pair string) {
	FallbackActivationsTotal.WithLabelValues(reason, pair).Inc()
}

// RecordFallbackDuration records the duration of a fallback operation
func RecordFallbackDuration(pair string, duration float64) {
	FallbackDuration.WithLabelValues(pair).Observe(duration)
}

// UpdateWebSocketConnectionStatus updates WebSocket connection status
func UpdateWebSocketConnectionStatus(connected bool) {
	status := 0.0
	if connected {
		status = 1.0
	}
	WebSocketConnectionStatus.WithLabelValues().Set(status)
}

// UpdateCircuitBreakerState updates circuit breaker state
// state: 0=closed, 1=open, 2=half_open
func UpdateCircuitBreakerState(service, endpoint string, state int) {
	CircuitBreakerState.WithLabelValues(service, endpoint).Set(float64(state))
}

// RecordWebSocketReconnectionAttempt records WebSocket reconnection attempts
func RecordWebSocketReconnectionAttempt(reason string) {
	WebSocketReconnectionAttempts.WithLabelValues(reason).Inc()
}
