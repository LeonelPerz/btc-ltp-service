package logger

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// StartTimeKey is the context key for start time
	StartTimeKey contextKey = "start_time"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	log.SetLevel(logrus.InfoLevel)
}

// GetLogger returns the singleton logger instance
func GetLogger() *logrus.Logger {
	return log
}

// SetLogLevel sets the global log level
func SetLogLevel(level string) {
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context) context.Context {
	requestID := uuid.New().String()
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithStartTime adds start time to the context
func WithStartTime(ctx context.Context) context.Context {
	return context.WithValue(ctx, StartTimeKey, time.Now())
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetStartTime extracts start time from context
func GetStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value(StartTimeKey).(time.Time); ok {
		return startTime
	}
	return time.Time{}
}

// LogHTTPRequest logs HTTP request information
func LogHTTPRequest(ctx context.Context, method, path, userAgent, remoteAddr string) {
	logger := log.WithFields(logrus.Fields{
		"request_id":  GetRequestID(ctx),
		"method":      method,
		"path":        path,
		"user_agent":  userAgent,
		"remote_addr": remoteAddr,
		"event":       "http_request",
	})
	logger.Info("HTTP request received")
}

// LogHTTPResponse logs HTTP response information
func LogHTTPResponse(ctx context.Context, statusCode int, responseSize int64) {
	startTime := GetStartTime(ctx)
	var latency time.Duration
	if !startTime.IsZero() {
		latency = time.Since(startTime)
	}

	logger := log.WithFields(logrus.Fields{
		"request_id":    GetRequestID(ctx),
		"status_code":   statusCode,
		"response_size": responseSize,
		"latency_ms":    latency.Milliseconds(),
		"latency_ns":    latency.Nanoseconds(),
		"event":         "http_response",
	})

	if statusCode >= 500 {
		logger.Error("HTTP response sent")
	} else if statusCode >= 400 {
		logger.Warn("HTTP response sent")
	} else {
		logger.Info("HTTP response sent")
	}
}

// LogKrakenRequest logs Kraken API request
func LogKrakenRequest(ctx context.Context, pairs []string, url string) {
	logger := log.WithFields(logrus.Fields{
		"request_id": GetRequestID(ctx),
		"pairs":      pairs,
		"url":        url,
		"event":      "kraken_request",
		"service":    "kraken_client",
	})
	logger.Info("Making request to Kraken API")
}

// LogKrakenResponse logs Kraken API response
func LogKrakenResponse(ctx context.Context, statusCode int, duration time.Duration, pairsCount int) {
	logger := log.WithFields(logrus.Fields{
		"request_id":           GetRequestID(ctx),
		"status_code":          statusCode,
		"upstream_duration_ms": duration.Milliseconds(),
		"upstream_duration_ns": duration.Nanoseconds(),
		"pairs_retrieved":      pairsCount,
		"event":                "kraken_response",
		"service":              "kraken_client",
	})

	if statusCode >= 500 {
		logger.Error("Kraken API response received")
	} else if statusCode >= 400 {
		logger.Warn("Kraken API response received")
	} else {
		logger.Info("Kraken API response received")
	}
}

// LogKrakenError logs Kraken API errors
func LogKrakenError(ctx context.Context, err error, attempt int, maxRetries int) {
	logger := log.WithFields(logrus.Fields{
		"request_id":  GetRequestID(ctx),
		"error":       err.Error(),
		"attempt":     attempt,
		"max_retries": maxRetries,
		"event":       "kraken_error",
		"service":     "kraken_client",
	})
	logger.Error("Kraken API error occurred")
}

// LogCacheOperation logs cache operations
func LogCacheOperation(ctx context.Context, operation string, backend string, pairs []string, hit bool, duration time.Duration) {
	logger := log.WithFields(logrus.Fields{
		"request_id":  GetRequestID(ctx),
		"operation":   operation,
		"backend":     backend,
		"pairs":       pairs,
		"cache_hit":   hit,
		"duration_ms": duration.Milliseconds(),
		"event":       "cache_operation",
		"service":     "cache",
	})
	logger.Info("Cache operation completed")
}

// LogPriceRefresh logs price refresh operations
func LogPriceRefresh(ctx context.Context, pairsCount int, duration time.Duration, success bool, err error) {
	fields := logrus.Fields{
		"request_id":  GetRequestID(ctx),
		"pairs_count": pairsCount,
		"duration_ms": duration.Milliseconds(),
		"success":     success,
		"event":       "price_refresh",
		"service":     "ltp_service",
	}

	if err != nil {
		fields["error"] = err.Error()
	}

	logger := log.WithFields(fields)

	if success {
		logger.Info("Price refresh completed successfully")
	} else {
		logger.Error("Price refresh failed")
	}
}

// LogServiceEvent logs general service events
func LogServiceEvent(ctx context.Context, event string, message string, fields map[string]interface{}) {
	logFields := logrus.Fields{
		"request_id": GetRequestID(ctx),
		"event":      event,
		"message":    message,
	}

	// Merge additional fields
	for k, v := range fields {
		logFields[k] = v
	}

	log.WithFields(logFields).Info(message)
}
