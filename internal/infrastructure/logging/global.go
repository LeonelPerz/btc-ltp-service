package logging

import (
	"context"
)

// Funciones globales de conveniencia para mantener compatibilidad
// con el código existente. Estas funciones usan el logger global por defecto.

// Debug logs a debug message using the global logger
func Debug(ctx context.Context, message string, fields Fields) {
	GetGlobalLogger().Debug(ctx, message, fields)
}

// Info logs an info message using the global logger
func Info(ctx context.Context, message string, fields Fields) {
	GetGlobalLogger().Info(ctx, message, fields)
}

// Warn logs a warning message using the global logger
func Warn(ctx context.Context, message string, fields Fields) {
	GetGlobalLogger().Warn(ctx, message, fields)
}

// Error logs an error message using the global logger
func Error(ctx context.Context, message string, fields Fields) {
	GetGlobalLogger().Error(ctx, message, fields)
}

// InfoWithError logs an info message with error details using the global logger
func InfoWithError(ctx context.Context, message string, err error, fields Fields) {
	GetGlobalLogger().InfoWithError(ctx, message, err, fields)
}

// WarnWithError logs a warning message with error details using the global logger
func WarnWithError(ctx context.Context, message string, err error, fields Fields) {
	GetGlobalLogger().WarnWithError(ctx, message, err, fields)
}

// ErrorWithError logs an error message with error details using the global logger
func ErrorWithError(ctx context.Context, message string, err error, fields Fields) {
	GetGlobalLogger().ErrorWithError(ctx, message, err, fields)
}

// HTTPRequest logs HTTP request details using the global HTTP logger
func HTTPRequest(ctx context.Context, method, path string, statusCode int, fields Fields) {
	// Mantener compatibilidad con la función anterior usando el HTTPLogger especializado
	duration := float64(0)
	if fields != nil {
		if d, ok := fields[FieldDuration]; ok {
			if df, ok := d.(float64); ok {
				duration = df
			}
		}
	}

	httpLogger := GetGlobalLoggers().HTTP
	httpLogger.RequestCompleted(ctx, method, path, statusCode, duration)
}

// ExternalRequest logs external API request details using the global external API logger
func ExternalRequest(ctx context.Context, service, endpoint string, durationMs float64, statusCode int, fields Fields) {
	externalLogger := GetGlobalLoggers().ExternalAPI
	externalLogger.RequestCompleted(ctx, service, endpoint, statusCode, durationMs)
}

// CacheOperation logs cache operations using the global cache logger
func CacheOperation(ctx context.Context, operation, key string, hit bool, fields Fields) {
	cacheLogger := GetGlobalLoggers().Cache
	if hit {
		cacheLogger.Hit(ctx, key, operation)
	} else {
		cacheLogger.Miss(ctx, key, operation)
	}
}

// SetLogLevel sets the global log level (mantiene compatibilidad)
func SetLogLevel(level LogLevel) {
	SetGlobalLogLevel(level)
}

// Funciones de conveniencia para obtener loggers especializados globales

// HTTP retorna el logger HTTP global
func HTTP() HTTPLogger {
	return GetGlobalLoggers().HTTP
}

// ExternalAPI retorna el logger de API externa global
func ExternalAPI() ExternalAPILogger {
	return GetGlobalLoggers().ExternalAPI
}

// Cache retorna el logger de cache global
func Cache() CacheLogger {
	return GetGlobalLoggers().Cache
}

// Business retorna el logger de negocio global
func Business() BusinessLogger {
	return GetGlobalLoggers().Business
}

// Security retorna el logger de seguridad global
func Security() SecurityLogger {
	return GetGlobalLoggers().Security
}

// Base retorna el logger base global
func Base() Logger {
	return GetGlobalLoggers().Base
}
