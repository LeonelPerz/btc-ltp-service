package middleware

import (
	"net/http"
	"time"

	"btc-ltp-service/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Record metrics
		duration := time.Since(start)
		endpoint := getEndpoint(r.URL.Path)

		metrics.RecordHTTPRequest(
			r.Method,
			endpoint,
			wrapped.statusCode,
			duration,
		)
	})
}

// getEndpoint normalizes URL paths to avoid high cardinality in metrics
func getEndpoint(path string) string {
	switch path {
	case "/api/v1/ltp":
		return "/api/v1/ltp"
	case "/api/v1/pairs":
		return "/api/v1/pairs"
	case "/health":
		return "/health"
	case "/metrics":
		return "/metrics"
	default:
		if len(path) > 0 && path[0] == '/' {
			return "/unknown"
		}
		return path
	}
}
