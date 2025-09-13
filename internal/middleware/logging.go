package middleware

import (
	"bytes"
	"net/http"

	"btc-ltp-service/internal/logger"
)

// responseWriter wraps http.ResponseWriter to capture response data
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *loggingResponseWriter) Write(data []byte) (int, error) {
	// Capture the response body for logging (optional, be careful with large responses)
	if rw.body != nil && len(data) < 1024 { // Only capture small responses
		rw.body.Write(data)
	}
	return rw.ResponseWriter.Write(data)
}

// LoggingMiddleware provides structured logging for HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add request ID and start time to context
		ctx := logger.WithRequestID(r.Context())
		ctx = logger.WithStartTime(ctx)
		r = r.WithContext(ctx)

		// Log incoming request
		logger.LogHTTPRequest(ctx, r.Method, r.URL.Path, r.UserAgent(), r.RemoteAddr)

		// Wrap response writer
		wrapped := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
			body:           new(bytes.Buffer),
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Calculate response size (approximate)
		responseSize := int64(wrapped.body.Len())

		// Log response
		logger.LogHTTPResponse(ctx, wrapped.statusCode, responseSize)
	})
}
