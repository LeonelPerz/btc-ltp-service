package middleware

import (
	"btc-ltp-service/internal/infrastructure/logging"
	"net/http"
	"time"
)

// ResponseWriter wrapper to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// RequestTracingMiddleware adds request tracing and structured logging
func RequestTracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate unique request ID
		requestID := logging.GenerateRequestID()

		// Create context with request ID and start time
		startTime := time.Now()
		ctx := logging.WithRequestID(r.Context(), requestID)
		ctx = logging.WithStartTime(ctx, startTime)

		// Add request ID to response headers (useful for debugging)
		w.Header().Set("X-Request-ID", requestID)

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     0,
		}

		// Extract request information
		method := r.Method
		path := r.URL.Path
		userAgent := r.Header.Get("User-Agent")
		remoteIP := getRemoteIP(r)

		// Log request start
		logging.Info(ctx, "HTTP request started", logging.Fields{
			"http_method":    method,
			"http_path":      path,
			"user_agent":     userAgent,
			"remote_ip":      remoteIP,
			"content_length": r.ContentLength,
		})

		// Process request with enriched context
		r = r.WithContext(ctx)
		next.ServeHTTP(wrapped, r)

		// Calculate response time
		duration := time.Since(startTime)
		durationMs := float64(duration.Nanoseconds()) / 1e6

		// Log request completion
		logging.HTTPRequest(ctx, method, path, wrapped.statusCode, logging.Fields{
			"user_agent":       userAgent,
			"remote_ip":        remoteIP,
			"response_size":    wrapped.written,
			"request_size":     r.ContentLength,
			"response_time_ms": durationMs,
		})
	})
}

// getRemoteIP extracts the real client IP from request
func getRemoteIP(r *http.Request) string {
	// Check X-Forwarded-For header (proxy)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		return xForwardedFor
	}

	// Check X-Real-IP header
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	// Fallback to remote address
	return r.RemoteAddr
}
