package ratelimit

import (
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/metrics"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Configuration constants with sensible defaults
const (
	DefaultCapacity   = 100 // 100 requests per bucket
	DefaultRefillRate = 10  // 10 requests per second refill
)

// RateLimitMiddleware provides rate limiting for HTTP requests
type RateLimitMiddleware struct {
	limiter   *RateLimiterCollection
	skipPaths map[string]bool
	enabled   bool
}

// NewRateLimitMiddleware creates a new rate limiting middleware (backward compatibility)
func NewRateLimitMiddleware() *RateLimitMiddleware {
	// Get configuration from environment
	capacity := getEnvIntOrDefault("RATE_LIMIT_CAPACITY", DefaultCapacity)
	refillRate := getEnvIntOrDefault("RATE_LIMIT_REFILL_RATE", DefaultRefillRate)
	enabled := getEnvOrDefault("RATE_LIMIT_ENABLED", "true") == "true"

	// Paths that should skip rate limiting
	skipPaths := map[string]bool{
		"/health":  true,
		"/ready":   true,
		"/metrics": true,
	}

	return &RateLimitMiddleware{
		limiter:   NewRateLimiterCollection(capacity, refillRate),
		skipPaths: skipPaths,
		enabled:   enabled,
	}
}

// NewRateLimitMiddlewareWithConfig creates a new rate limiting middleware with configuration
func NewRateLimitMiddlewareWithConfig(rateLimitConfig config.RateLimitConfig) *RateLimitMiddleware {
	// Paths that should skip rate limiting
	skipPaths := map[string]bool{
		"/health":  true,
		"/ready":   true,
		"/metrics": true,
	}

	var limiter *RateLimiterCollection
	if rateLimitConfig.Enabled {
		limiter = NewRateLimiterCollection(rateLimitConfig.Capacity, rateLimitConfig.RefillRate)
	}

	return &RateLimitMiddleware{
		limiter:   limiter,
		skipPaths: skipPaths,
		enabled:   rateLimitConfig.Enabled,
	}
}

// Handler returns the HTTP middleware handler
func (rlm *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if disabled
		if !rlm.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip rate limiting for certain paths
		if rlm.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		clientID := getClientID(r)

		// Check rate limit
		allowed := rlm.limiter.Allow(clientID)
		tokensRemaining := rlm.limiter.Tokens(clientID)

		// Record metrics
		metrics.RecordRateLimitResult(allowed)
		metrics.UpdateRateLimitTokens(clientID, float64(tokensRemaining))

		if !allowed {
			// Rate limit exceeded
			logging.Warn(ctx, "Rate limit exceeded", logging.Fields{
				"client_id":  clientID,
				"path":       r.URL.Path,
				"method":     r.Method,
				"user_agent": r.Header.Get("User-Agent"),
			})

			rlm.writeRateLimitError(w, r)
			return
		}

		// Add rate limit headers to response
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(tokensRemaining))

		// Continue with request
		next.ServeHTTP(w, r)
	})
}

// getClientID extracts a client identifier from the request
// This is used as the key for rate limiting buckets
func getClientID(r *http.Request) string {
	// Try to get real IP from headers (reverse proxy/load balancer)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		parts := strings.Split(xForwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	// Fallback to RemoteAddr
	remoteAddr := r.RemoteAddr

	// Remove port if present (IPv4)
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// writeRateLimitError writes a rate limit exceeded error response
func (rlm *RateLimitMiddleware) writeRateLimitError(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("Retry-After", "1") // Suggest retry after 1 second
	w.WriteHeader(http.StatusTooManyRequests)

	errorResponse := map[string]interface{}{
		"error":   "RATE_LIMIT_EXCEEDED",
		"message": "Rate limit exceeded. Please slow down your requests.",
		"code":    http.StatusTooManyRequests,
		"details": map[string]interface{}{
			"retry_after_seconds": 1,
			"limit_info":          "Please reduce your request rate and try again",
		},
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// Stats returns rate limiting statistics
func (rlm *RateLimitMiddleware) Stats() map[string]interface{} {
	stats := rlm.limiter.Stats()
	stats["enabled"] = rlm.enabled
	return stats
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
