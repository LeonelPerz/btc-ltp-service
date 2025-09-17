package metrics

import (
	"net/http"
	"strings"
	"time"
)

// HTTPMetricsMiddleware collects HTTP metrics for Prometheus
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Wrap response writer to capture metrics
		wrapped := &responseWriterMetrics{
			ResponseWriter: w,
			statusCode:     200, // Default to 200 if WriteHeader is not called
			written:        0,
		}

		// Extract normalized path (to avoid high cardinality)
		normalizedPath := normalizePath(r.URL.Path)
		method := r.Method

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate metrics
		duration := time.Since(startTime).Seconds()
		statusCode := wrapped.statusCode
		requestSize := r.ContentLength
		responseSize := wrapped.written

		// Record metrics
		RecordHTTPRequest(method, normalizedPath, statusCode, duration, requestSize, responseSize)
	})
}

// responseWriterMetrics wraps http.ResponseWriter to capture metrics
type responseWriterMetrics struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// WriteHeader captures the status code
func (rw *responseWriterMetrics) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size
func (rw *responseWriterMetrics) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = 200
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// normalizePath normalizes URL paths to avoid high cardinality in metrics
// This is important to prevent metrics explosion from dynamic paths
func normalizePath(path string) string {
	// Handle root path
	if path == "/" {
		return "/"
	}

	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// Handle common API patterns
	switch {
	case path == "/health":
		return "/health"
	case path == "/ready":
		return "/ready"
	case path == "/metrics":
		return "/metrics"
	case strings.HasPrefix(path, "/api/v1/ltp/cached"):
		return "/api/v1/ltp/cached"
	case strings.HasPrefix(path, "/api/v1/ltp/refresh"):
		return "/api/v1/ltp/refresh"
	case strings.HasPrefix(path, "/api/v1/ltp"):
		return "/api/v1/ltp"
	case strings.HasPrefix(path, "/api/v1/"):
		return "/api/v1/*"
	case strings.HasPrefix(path, "/api/"):
		return "/api/*"
	default:
		// For unknown paths, use a generic label
		return "/unknown"
	}
}
