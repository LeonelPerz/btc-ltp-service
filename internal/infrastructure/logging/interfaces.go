package logging

import (
	"context"
)

// Logger define la interfaz principal para logging estructurado
type Logger interface {
	// Métodos básicos de logging por nivel
	Debug(ctx context.Context, message string, fields Fields)
	Info(ctx context.Context, message string, fields Fields)
	Warn(ctx context.Context, message string, fields Fields)
	Error(ctx context.Context, message string, fields Fields)

	// Métodos con error incluido
	InfoWithError(ctx context.Context, message string, err error, fields Fields)
	WarnWithError(ctx context.Context, message string, err error, fields Fields)
	ErrorWithError(ctx context.Context, message string, err error, fields Fields)

	// Configuración
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// DomainLogger representa loggers especializados por dominio
type DomainLogger interface {
	Logger

	// Identificador del dominio
	Domain() string
}

// HTTPLogger especializado para logs relacionados con HTTP
type HTTPLogger interface {
	DomainLogger

	RequestReceived(ctx context.Context, method, path, userAgent, remoteIP string)
	RequestCompleted(ctx context.Context, method, path string, statusCode int, duration float64)
	RequestFailed(ctx context.Context, method, path string, statusCode int, err error, duration float64)
}

// ExternalAPILogger especializado para logs de APIs externas
type ExternalAPILogger interface {
	DomainLogger

	RequestStarted(ctx context.Context, service, endpoint, method string)
	RequestCompleted(ctx context.Context, service, endpoint string, statusCode int, duration float64)
	RequestFailed(ctx context.Context, service, endpoint string, statusCode int, err error, duration float64)
}

// CacheLogger especializado para logs relacionados con cache
type CacheLogger interface {
	DomainLogger

	Hit(ctx context.Context, key string, operation string)
	Miss(ctx context.Context, key string, operation string)
	Set(ctx context.Context, key string, ttl float64)
	Delete(ctx context.Context, key string)
	CacheError(ctx context.Context, operation, key string, err error)
}

// BusinessLogger especializado para logs de lógica de negocio
type BusinessLogger interface {
	DomainLogger

	PriceRequested(ctx context.Context, pair string, source string)
	PriceServed(ctx context.Context, pair string, price float64, source string, cached bool)
	PriceUpdateFailed(ctx context.Context, pair string, err error)
	ValidationFailed(ctx context.Context, input string, reason string)
}

// SecurityLogger especializado para logs relacionados con seguridad
type SecurityLogger interface {
	DomainLogger

	RateLimitExceeded(ctx context.Context, clientIP string, endpoint string)
	InvalidRequest(ctx context.Context, clientIP string, reason string)
	SuspiciousActivity(ctx context.Context, clientIP string, activity string)
}
