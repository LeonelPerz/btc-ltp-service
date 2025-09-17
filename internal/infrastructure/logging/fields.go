package logging

import (
	"context"
	"time"
)

// Fields representa campos estructurados para logs
type Fields map[string]interface{}

// LogLevel representa los diferentes niveles de log
type LogLevel string

// Niveles de log disponibles
const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

// Campos estándar para logs
const (
	FieldTimestamp  = "timestamp"
	FieldLevel      = "level"
	FieldMessage    = "message"
	FieldRequestID  = "request_id"
	FieldService    = "service"
	FieldVersion    = "version"
	FieldDomain     = "domain"
	FieldError      = "error"
	FieldErrorType  = "error_type"
	FieldDuration   = "duration_ms"
	FieldStatusCode = "status_code"
	FieldUserAgent  = "user_agent"
	FieldRemoteIP   = "remote_ip"
	FieldMethod     = "method"
	FieldPath       = "path"
	FieldQuery      = "query"
	FieldHeaders    = "headers"
)

// Campos para contexto de requests
const (
	FieldHTTPMethod     = "http_method"
	FieldHTTPPath       = "http_path"
	FieldHTTPStatusCode = "http_status_code"
	FieldHTTPUserAgent  = "http_user_agent"
	FieldHTTPRemoteIP   = "http_remote_ip"
	FieldHTTPHeaders    = "http_headers"
	FieldHTTPQuery      = "http_query"
)

// Campos para APIs externas
const (
	FieldExternalService  = "external_service"
	FieldExternalEndpoint = "external_endpoint"
	FieldExternalMethod   = "external_method"
	FieldExternalStatus   = "external_status_code"
	FieldExternalDuration = "external_duration_ms"
)

// Campos para cache
const (
	FieldCacheOperation = "cache_operation"
	FieldCacheKey       = "cache_key"
	FieldCacheHit       = "cache_hit"
	FieldCacheTTL       = "cache_ttl_seconds"
)

// Campos para lógica de negocio
const (
	FieldPair       = "pair"
	FieldPrice      = "price"
	FieldSource     = "source"
	FieldCached     = "cached"
	FieldValidation = "validation"
)

// Campos para seguridad
const (
	FieldClientIP         = "client_ip"
	FieldSuspiciousReason = "suspicious_reason"
	FieldRateLimit        = "rate_limit"
)

// Operaciones de cache
const (
	CacheOpGet    = "GET"
	CacheOpSet    = "SET"
	CacheOpDelete = "DELETE"
	CacheOpClear  = "CLEAR"
)

// FieldBuilder ayuda a construir campos de manera estandarizada
type FieldBuilder struct {
	fields Fields
}

// NewFieldBuilder crea un nuevo builder de campos
func NewFieldBuilder() *FieldBuilder {
	return &FieldBuilder{
		fields: make(Fields),
	}
}

// WithError añade información del error
func (fb *FieldBuilder) WithError(err error) *FieldBuilder {
	if err != nil {
		fb.fields[FieldError] = err.Error()
		fb.fields[FieldErrorType] = getErrorType(err)
	}
	return fb
}

// WithDuration añade duración en milliseconds
func (fb *FieldBuilder) WithDuration(duration time.Duration) *FieldBuilder {
	fb.fields[FieldDuration] = float64(duration.Nanoseconds()) / 1e6
	return fb
}

// WithHTTPInfo añade información HTTP básica
func (fb *FieldBuilder) WithHTTPInfo(method, path string, statusCode int) *FieldBuilder {
	fb.fields[FieldHTTPMethod] = method
	fb.fields[FieldHTTPPath] = path
	fb.fields[FieldHTTPStatusCode] = statusCode
	return fb
}

// WithUserAgent añade user agent
func (fb *FieldBuilder) WithUserAgent(userAgent string) *FieldBuilder {
	if userAgent != "" {
		fb.fields[FieldHTTPUserAgent] = userAgent
	}
	return fb
}

// WithRemoteIP añade IP remota
func (fb *FieldBuilder) WithRemoteIP(ip string) *FieldBuilder {
	if ip != "" {
		fb.fields[FieldHTTPRemoteIP] = ip
	}
	return fb
}

// WithExternalAPI añade información de API externa
func (fb *FieldBuilder) WithExternalAPI(service, endpoint string, statusCode int, duration time.Duration) *FieldBuilder {
	fb.fields[FieldExternalService] = service
	fb.fields[FieldExternalEndpoint] = endpoint
	fb.fields[FieldExternalStatus] = statusCode
	fb.fields[FieldExternalDuration] = float64(duration.Nanoseconds()) / 1e6
	return fb
}

// WithCache añade información de cache
func (fb *FieldBuilder) WithCache(operation, key string, hit bool) *FieldBuilder {
	fb.fields[FieldCacheOperation] = operation
	fb.fields[FieldCacheKey] = key
	fb.fields[FieldCacheHit] = hit
	return fb
}

// WithBusinessContext añade contexto de negocio
func (fb *FieldBuilder) WithBusinessContext(pair string, price float64, source string, cached bool) *FieldBuilder {
	fb.fields[FieldPair] = pair
	if price > 0 {
		fb.fields[FieldPrice] = price
	}
	fb.fields[FieldSource] = source
	fb.fields[FieldCached] = cached
	return fb
}

// WithCustomField añade un campo personalizado
func (fb *FieldBuilder) WithCustomField(key string, value interface{}) *FieldBuilder {
	if key != "" && value != nil {
		fb.fields[key] = value
	}
	return fb
}

// Build retorna los campos construidos
func (fb *FieldBuilder) Build() Fields {
	if len(fb.fields) == 0 {
		return nil
	}
	return fb.fields
}

// Context keys para información del request
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	StartTimeKey contextKey = "start_time"
	UserAgentKey contextKey = "user_agent"
	RemoteIPKey  contextKey = "remote_ip"
)

// Funciones de utilidad para contexto
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

func WithStartTime(ctx context.Context, startTime time.Time) context.Context {
	return context.WithValue(ctx, StartTimeKey, startTime)
}

func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, UserAgentKey, userAgent)
}

func WithRemoteIP(ctx context.Context, remoteIP string) context.Context {
	return context.WithValue(ctx, RemoteIPKey, remoteIP)
}

func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

func GetStartTime(ctx context.Context) time.Time {
	if startTime, ok := ctx.Value(StartTimeKey).(time.Time); ok {
		return startTime
	}
	return time.Time{}
}

func GetUserAgent(ctx context.Context) string {
	if userAgent, ok := ctx.Value(UserAgentKey).(string); ok {
		return userAgent
	}
	return ""
}

func GetRemoteIP(ctx context.Context) string {
	if remoteIP, ok := ctx.Value(RemoteIPKey).(string); ok {
		return remoteIP
	}
	return ""
}

// getErrorType extrae el tipo de error para logging
func getErrorType(err error) string {
	if err == nil {
		return ""
	}
	return err.Error() // Simplificado por ahora, se puede mejorar con reflection
}
