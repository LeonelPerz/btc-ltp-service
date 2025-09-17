package middleware

import (
	"btc-ltp-service/internal/infrastructure/logging"
	"net/http"
)

// LoggingMiddleware provides enhanced logging capabilities using domain-specific loggers
// Note: This middleware complements RequestTracingMiddleware
// RequestTracingMiddleware handles the main request/response logging
// This middleware provides detailed debugging and security logging
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Obtener loggers especializados
		httpLogger := logging.HTTP()
		securityLogger := logging.Security()

		// Log del inicio del request con información detallada
		httpLogger.RequestReceived(ctx, r.Method, r.URL.Path, r.UserAgent(), r.RemoteAddr)

		// Log información adicional de debug si es necesario
		logging.Debug(ctx, "Processing HTTP request", logging.Fields{
			"headers":        extractImportantHeaders(r),
			"query":          r.URL.RawQuery,
			"content_length": r.ContentLength,
		})

		// Detectar requests potencialmente sospechosos
		if isSuspiciousRequest(r) {
			securityLogger.SuspiciousActivity(ctx, r.RemoteAddr, "unusual_request_pattern")
		}

		// Process request
		next.ServeHTTP(w, r)

		// Additional post-processing logging can be added here
		logging.Debug(ctx, "HTTP request processing completed", nil)
	})
}

// extractImportantHeaders extracts relevant headers for logging
func extractImportantHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// Log important headers (avoid sensitive data)
	importantHeaders := []string{
		"Content-Type",
		"Accept",
		"Accept-Encoding",
		"Cache-Control",
		"X-Forwarded-For",
		"X-Real-IP",
	}

	for _, header := range importantHeaders {
		if value := r.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	return headers
}

// isSuspiciousRequest detecta patrones sospechosos en las requests
func isSuspiciousRequest(r *http.Request) bool {
	// Detectar patrones comunes de ataques
	path := r.URL.Path
	query := r.URL.RawQuery

	// Patrones sospechosos comunes
	suspiciousPatterns := []string{
		"../",
		"<script",
		"SELECT",
		"UNION",
		"DROP",
		"INSERT",
		"UPDATE",
		"DELETE",
		"exec(",
		"eval(",
	}

	for _, pattern := range suspiciousPatterns {
		if containsIgnoreCase(path, pattern) || containsIgnoreCase(query, pattern) {
			return true
		}
	}

	// Content-Length inusualmente grande
	if r.ContentLength > 1024*1024 { // 1MB
		return true
	}

	return false
}

// containsIgnoreCase verifica si una cadena contiene otra ignorando mayúsculas/minúsculas
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		findIgnoreCase(s, substr) != -1
}

// findIgnoreCase busca una subcadena ignorando mayúsculas/minúsculas
func findIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// toLower convierte un byte a minúscula
func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
