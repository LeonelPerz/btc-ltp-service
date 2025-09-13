package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_http_requests_total",
			Help: "The total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_http_request_duration_seconds",
			Help:    "The HTTP request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Cache metrics
	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_cache_hits_total",
			Help: "The total number of cache hits",
		},
		[]string{"cache_backend"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_cache_misses_total",
			Help: "The total number of cache misses",
		},
		[]string{"cache_backend"},
	)

	CacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_cache_operation_duration_seconds",
			Help:    "The cache operation latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"cache_backend", "operation"},
	)

	// Kraken API metrics
	KrakenRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_kraken_requests_total",
			Help: "The total number of Kraken API requests",
		},
		[]string{"status_code"},
	)

	KrakenRequestDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_kraken_request_duration_seconds",
			Help:    "The Kraken API request latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	KrakenRetries = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "btc_ltp_kraken_retries_total",
			Help: "The total number of Kraken API request retries",
		},
	)

	KrakenErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_kraken_errors_total",
			Help: "The total number of Kraken API errors",
		},
		[]string{"error_type"},
	)

	// Price refresh metrics
	PriceRefreshTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "btc_ltp_price_refresh_total",
			Help: "The total number of price refresh operations",
		},
		[]string{"status"},
	)

	PriceRefreshDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "btc_ltp_price_refresh_duration_seconds",
			Help:    "The price refresh operation latencies in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Current price info
	CurrentPrices = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_current_price",
			Help: "The current price for trading pairs",
		},
		[]string{"pair"},
	)

	// Service info
	ServiceInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "btc_ltp_service_info",
			Help: "Information about the BTC LTP service",
		},
		[]string{"version", "cache_backend"},
	)
)

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	HTTPRequestsTotal.WithLabelValues(method, endpoint, string(rune(statusCode+'0'))).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordCacheHit records a cache hit
func RecordCacheHit(backend string) {
	CacheHitsTotal.WithLabelValues(backend).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(backend string) {
	CacheMissesTotal.WithLabelValues(backend).Inc()
}

// RecordCacheOperation records cache operation duration
func RecordCacheOperation(backend, operation string, duration time.Duration) {
	CacheOperationDuration.WithLabelValues(backend, operation).Observe(duration.Seconds())
}

// RecordKrakenRequest records Kraken API request metrics
func RecordKrakenRequest(statusCode int, duration time.Duration) {
	KrakenRequestsTotal.WithLabelValues(string(rune(statusCode + '0'))).Inc()
	KrakenRequestDuration.Observe(duration.Seconds())
}

// RecordKrakenRetry records a Kraken API retry
func RecordKrakenRetry() {
	KrakenRetries.Inc()
}

// RecordKrakenError records a Kraken API error
func RecordKrakenError(errorType string) {
	KrakenErrors.WithLabelValues(errorType).Inc()
}

// RecordPriceRefresh records price refresh operation
func RecordPriceRefresh(status string, duration time.Duration) {
	PriceRefreshTotal.WithLabelValues(status).Inc()
	PriceRefreshDuration.Observe(duration.Seconds())
}

// UpdateCurrentPrice updates the current price gauge
func UpdateCurrentPrice(pair string, price float64) {
	CurrentPrices.WithLabelValues(pair).Set(price)
}

// SetServiceInfo sets service information
func SetServiceInfo(version, cacheBackend string) {
	ServiceInfo.WithLabelValues(version, cacheBackend).Set(1)
}
