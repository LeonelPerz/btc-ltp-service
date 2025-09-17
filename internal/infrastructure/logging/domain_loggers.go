package logging

import (
	"context"
)

// BaseDomainLogger implementa funcionalidad común para loggers de dominio
type BaseDomainLogger struct {
	Logger
	domain string
}

// Domain retorna el dominio del logger
func (dl *BaseDomainLogger) Domain() string {
	return dl.domain
}

// logWithDomain agrega el campo de dominio a los logs
func (dl *BaseDomainLogger) logWithDomain(ctx context.Context, level LogLevel, message string, fields Fields) {
	if fields == nil {
		fields = make(Fields)
	}
	fields[FieldDomain] = dl.domain

	switch level {
	case LevelDebug:
		dl.Logger.Debug(ctx, message, fields)
	case LevelInfo:
		dl.Logger.Info(ctx, message, fields)
	case LevelWarn:
		dl.Logger.Warn(ctx, message, fields)
	case LevelError:
		dl.Logger.Error(ctx, message, fields)
	}
}

// Override métodos base para incluir dominio
func (dl *BaseDomainLogger) Debug(ctx context.Context, message string, fields Fields) {
	dl.logWithDomain(ctx, LevelDebug, message, fields)
}

func (dl *BaseDomainLogger) Info(ctx context.Context, message string, fields Fields) {
	dl.logWithDomain(ctx, LevelInfo, message, fields)
}

func (dl *BaseDomainLogger) Warn(ctx context.Context, message string, fields Fields) {
	dl.logWithDomain(ctx, LevelWarn, message, fields)
}

func (dl *BaseDomainLogger) Error(ctx context.Context, message string, fields Fields) {
	dl.logWithDomain(ctx, LevelError, message, fields)
}

// HTTPDomainLogger especializado para logs HTTP
type HTTPDomainLogger struct {
	*BaseDomainLogger
}

// NewHTTPLogger crea un nuevo logger HTTP
func NewHTTPLogger(baseLogger Logger) HTTPLogger {
	return &HTTPDomainLogger{
		BaseDomainLogger: &BaseDomainLogger{
			Logger: baseLogger,
			domain: "http",
		},
	}
}

func (hl *HTTPDomainLogger) RequestReceived(ctx context.Context, method, path, userAgent, remoteIP string) {
	fields := NewFieldBuilder().
		WithHTTPInfo(method, path, 0).
		WithUserAgent(userAgent).
		WithRemoteIP(remoteIP).
		Build()

	hl.Info(ctx, "HTTP request received", fields)
}

func (hl *HTTPDomainLogger) RequestCompleted(ctx context.Context, method, path string, statusCode int, duration float64) {
	fields := NewFieldBuilder().
		WithHTTPInfo(method, path, statusCode).
		WithCustomField(FieldDuration, duration).
		Build()

	level := LevelInfo
	if statusCode >= 400 && statusCode < 500 {
		level = LevelWarn
	} else if statusCode >= 500 {
		level = LevelError
	}

	message := "HTTP request completed"
	switch level {
	case LevelWarn:
		hl.logWithDomain(ctx, level, message, fields)
	case LevelError:
		hl.logWithDomain(ctx, level, message, fields)
	default:
		hl.Info(ctx, message, fields)
	}
}

func (hl *HTTPDomainLogger) RequestFailed(ctx context.Context, method, path string, statusCode int, err error, duration float64) {
	fields := NewFieldBuilder().
		WithHTTPInfo(method, path, statusCode).
		WithError(err).
		WithCustomField(FieldDuration, duration).
		Build()

	hl.ErrorWithError(ctx, "HTTP request failed", err, fields)
}

// ExternalAPIDomainLogger especializado para APIs externas
type ExternalAPIDomainLogger struct {
	*BaseDomainLogger
}

// NewExternalAPILogger crea un nuevo logger para APIs externas
func NewExternalAPILogger(baseLogger Logger) ExternalAPILogger {
	return &ExternalAPIDomainLogger{
		BaseDomainLogger: &BaseDomainLogger{
			Logger: baseLogger,
			domain: "external_api",
		},
	}
}

func (el *ExternalAPIDomainLogger) RequestStarted(ctx context.Context, service, endpoint, method string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldExternalService, service).
		WithCustomField(FieldExternalEndpoint, endpoint).
		WithCustomField(FieldExternalMethod, method).
		Build()

	el.Debug(ctx, "External API request started", fields)
}

func (el *ExternalAPIDomainLogger) RequestCompleted(ctx context.Context, service, endpoint string, statusCode int, duration float64) {
	fields := NewFieldBuilder().
		WithCustomField(FieldExternalService, service).
		WithCustomField(FieldExternalEndpoint, endpoint).
		WithCustomField(FieldExternalStatus, statusCode).
		WithCustomField(FieldExternalDuration, duration).
		Build()

	level := LevelInfo
	if statusCode >= 400 && statusCode < 500 {
		level = LevelWarn
	} else if statusCode >= 500 {
		level = LevelError
	}

	message := "External API request completed"
	el.logWithDomain(ctx, level, message, fields)
}

func (el *ExternalAPIDomainLogger) RequestFailed(ctx context.Context, service, endpoint string, statusCode int, err error, duration float64) {
	fields := NewFieldBuilder().
		WithCustomField(FieldExternalService, service).
		WithCustomField(FieldExternalEndpoint, endpoint).
		WithCustomField(FieldExternalStatus, statusCode).
		WithError(err).
		WithCustomField(FieldExternalDuration, duration).
		Build()

	el.ErrorWithError(ctx, "External API request failed", err, fields)
}

// CacheDomainLogger especializado para cache
type CacheDomainLogger struct {
	*BaseDomainLogger
}

// NewCacheLogger crea un nuevo logger de cache
func NewCacheLogger(baseLogger Logger) CacheLogger {
	return &CacheDomainLogger{
		BaseDomainLogger: &BaseDomainLogger{
			Logger: baseLogger,
			domain: "cache",
		},
	}
}

func (cl *CacheDomainLogger) Hit(ctx context.Context, key string, operation string) {
	fields := NewFieldBuilder().
		WithCache(operation, key, true).
		Build()

	cl.Debug(ctx, "Cache hit", fields)
}

func (cl *CacheDomainLogger) Miss(ctx context.Context, key string, operation string) {
	fields := NewFieldBuilder().
		WithCache(operation, key, false).
		Build()

	cl.Debug(ctx, "Cache miss", fields)
}

func (cl *CacheDomainLogger) Set(ctx context.Context, key string, ttl float64) {
	fields := NewFieldBuilder().
		WithCustomField(FieldCacheKey, key).
		WithCustomField(FieldCacheOperation, CacheOpSet).
		WithCustomField(FieldCacheTTL, ttl).
		Build()

	cl.Debug(ctx, "Cache set", fields)
}

func (cl *CacheDomainLogger) Delete(ctx context.Context, key string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldCacheKey, key).
		WithCustomField(FieldCacheOperation, CacheOpDelete).
		Build()

	cl.Debug(ctx, "Cache delete", fields)
}

func (cl *CacheDomainLogger) CacheError(ctx context.Context, operation, key string, err error) {
	fields := NewFieldBuilder().
		WithCustomField(FieldCacheOperation, operation).
		WithCustomField(FieldCacheKey, key).
		WithError(err).
		Build()

	cl.ErrorWithError(ctx, "Cache operation failed", err, fields)
}

// BusinessDomainLogger especializado para lógica de negocio
type BusinessDomainLogger struct {
	*BaseDomainLogger
}

// NewBusinessLogger crea un nuevo logger de negocio
func NewBusinessLogger(baseLogger Logger) BusinessLogger {
	return &BusinessDomainLogger{
		BaseDomainLogger: &BaseDomainLogger{
			Logger: baseLogger,
			domain: "business",
		},
	}
}

func (bl *BusinessDomainLogger) PriceRequested(ctx context.Context, pair string, source string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldPair, pair).
		WithCustomField(FieldSource, source).
		Build()

	bl.Info(ctx, "Price requested", fields)
}

func (bl *BusinessDomainLogger) PriceServed(ctx context.Context, pair string, price float64, source string, cached bool) {
	fields := NewFieldBuilder().
		WithBusinessContext(pair, price, source, cached).
		Build()

	bl.Info(ctx, "Price served", fields)
}

func (bl *BusinessDomainLogger) PriceUpdateFailed(ctx context.Context, pair string, err error) {
	fields := NewFieldBuilder().
		WithCustomField(FieldPair, pair).
		WithError(err).
		Build()

	bl.ErrorWithError(ctx, "Price update failed", err, fields)
}

func (bl *BusinessDomainLogger) ValidationFailed(ctx context.Context, input string, reason string) {
	fields := NewFieldBuilder().
		WithCustomField("input", input).
		WithCustomField("reason", reason).
		WithCustomField(FieldValidation, "failed").
		Build()

	bl.Warn(ctx, "Input validation failed", fields)
}

// SecurityDomainLogger especializado para seguridad
type SecurityDomainLogger struct {
	*BaseDomainLogger
}

// NewSecurityLogger crea un nuevo logger de seguridad
func NewSecurityLogger(baseLogger Logger) SecurityLogger {
	return &SecurityDomainLogger{
		BaseDomainLogger: &BaseDomainLogger{
			Logger: baseLogger,
			domain: "security",
		},
	}
}

func (sl *SecurityDomainLogger) RateLimitExceeded(ctx context.Context, clientIP string, endpoint string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldClientIP, clientIP).
		WithCustomField("endpoint", endpoint).
		WithCustomField(FieldRateLimit, "exceeded").
		Build()

	sl.Warn(ctx, "Rate limit exceeded", fields)
}

func (sl *SecurityDomainLogger) InvalidRequest(ctx context.Context, clientIP string, reason string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldClientIP, clientIP).
		WithCustomField("reason", reason).
		Build()

	sl.Warn(ctx, "Invalid request received", fields)
}

func (sl *SecurityDomainLogger) SuspiciousActivity(ctx context.Context, clientIP string, activity string) {
	fields := NewFieldBuilder().
		WithCustomField(FieldClientIP, clientIP).
		WithCustomField("activity", activity).
		WithCustomField(FieldSuspiciousReason, activity).
		Build()

	sl.Error(ctx, "Suspicious activity detected", fields)
}
