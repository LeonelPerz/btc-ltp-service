package logging

import (
	"fmt"
	"os"
)

// LoggerFactory facilita la creación de diferentes tipos de loggers
type LoggerFactory struct {
	baseLogger Logger
}

// NewLoggerFactory crea una nueva factory de loggers
func NewLoggerFactory(config *LoggerConfig) (*LoggerFactory, error) {
	if config == nil {
		config = DefaultConfig()
	}

	baseLogger, err := NewStructuredLogger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create base logger: %w", err)
	}

	return &LoggerFactory{
		baseLogger: baseLogger,
	}, nil
}

// NewLoggerFactoryWithDefaults crea una factory con configuración por defecto
func NewLoggerFactoryWithDefaults(service, version, environment string) (*LoggerFactory, error) {
	config := NewConfig(service, version, environment)
	return NewLoggerFactory(config)
}

// GetBaseLogger retorna el logger base
func (f *LoggerFactory) GetBaseLogger() Logger {
	return f.baseLogger
}

// GetHTTPLogger retorna un logger especializado para HTTP
func (f *LoggerFactory) GetHTTPLogger() HTTPLogger {
	return NewHTTPLogger(f.baseLogger)
}

// GetExternalAPILogger retorna un logger especializado para APIs externas
func (f *LoggerFactory) GetExternalAPILogger() ExternalAPILogger {
	return NewExternalAPILogger(f.baseLogger)
}

// GetCacheLogger retorna un logger especializado para cache
func (f *LoggerFactory) GetCacheLogger() CacheLogger {
	return NewCacheLogger(f.baseLogger)
}

// GetBusinessLogger retorna un logger especializado para lógica de negocio
func (f *LoggerFactory) GetBusinessLogger() BusinessLogger {
	return NewBusinessLogger(f.baseLogger)
}

// GetSecurityLogger retorna un logger especializado para seguridad
func (f *LoggerFactory) GetSecurityLogger() SecurityLogger {
	return NewSecurityLogger(f.baseLogger)
}

// UpdateLogLevel actualiza el nivel de log del logger base
func (f *LoggerFactory) UpdateLogLevel(level LogLevel) {
	f.baseLogger.SetLevel(level)
}

// LoggerSet contiene todos los loggers especializados
type LoggerSet struct {
	Base        Logger
	HTTP        HTTPLogger
	ExternalAPI ExternalAPILogger
	Cache       CacheLogger
	Business    BusinessLogger
	Security    SecurityLogger
}

// GetLoggerSet retorna un set completo de loggers especializados
func (f *LoggerFactory) GetLoggerSet() *LoggerSet {
	return &LoggerSet{
		Base:        f.baseLogger,
		HTTP:        f.GetHTTPLogger(),
		ExternalAPI: f.GetExternalAPILogger(),
		Cache:       f.GetCacheLogger(),
		Business:    f.GetBusinessLogger(),
		Security:    f.GetSecurityLogger(),
	}
}

// Global factory instance y variables globales para compatibilidad
var (
	globalFactory *LoggerFactory
	globalLoggers *LoggerSet
)

// InitializeGlobalLoggers inicializa los loggers globales
func InitializeGlobalLoggers(config *LoggerConfig) error {
	factory, err := NewLoggerFactory(config)
	if err != nil {
		return fmt.Errorf("failed to initialize global loggers: %w", err)
	}

	globalFactory = factory
	globalLoggers = factory.GetLoggerSet()
	return nil
}

// InitializeGlobalLoggersWithDefaults inicializa los loggers globales con configuración por defecto
func InitializeGlobalLoggersWithDefaults(service, version, environment string, level LogLevel) error {
	config := NewConfig(service, version, environment).WithLevel(level)
	return InitializeGlobalLoggers(config)
}

// GetGlobalLogger retorna el logger base global
func GetGlobalLogger() Logger {
	if globalLoggers == nil {
		// Fallback en caso de que no se hayan inicializado los loggers globales
		_ = InitializeGlobalLoggersWithDefaults("unknown-service", "1.0.0", "development", LevelInfo)
	}
	return globalLoggers.Base
}

// GetGlobalLoggers retorna todos los loggers globales
func GetGlobalLoggers() *LoggerSet {
	if globalLoggers == nil {
		// Fallback en caso de que no se hayan inicializado los loggers globales
		_ = InitializeGlobalLoggersWithDefaults("unknown-service", "1.0.0", "development", LevelInfo)
	}
	return globalLoggers
}

// SetGlobalLogLevel actualiza el nivel de log global
func SetGlobalLogLevel(level LogLevel) {
	if globalFactory != nil {
		globalFactory.UpdateLogLevel(level)
	}
}

// Funciones de conveniencia para crear configuraciones comunes

// NewDevelopmentConfig crea una configuración para desarrollo
func NewDevelopmentConfig(service string) *LoggerConfig {
	return NewConfig(service, "dev", "development").
		WithLevel(LevelDebug).
		WithFormat(FormatText).
		WithSource(true)
}

// NewProductionConfig crea una configuración para producción
func NewProductionConfig(service, version string) *LoggerConfig {
	return NewConfig(service, version, "production").
		WithLevel(LevelInfo).
		WithFormat(FormatJSON).
		WithSource(false)
}

// NewTestingConfig crea una configuración para testing
func NewTestingConfig(service string) *LoggerConfig {
	// En tests, podríamos querer capturar logs en un buffer para verificaciones
	return NewConfig(service, "test", "testing").
		WithLevel(LevelDebug).
		WithFormat(FormatJSON).
		WithOutput(os.Stdout).
		WithSource(true)
}

// ConfigFromEnvironment crea una configuración basada en variables de entorno
func ConfigFromEnvironment(service, version string) *LoggerConfig {
	config := NewConfig(service, version, getEnvOrDefault("ENVIRONMENT", "development"))

	// Configurar nivel desde variable de entorno
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.WithLevel(LogLevelFromString(level))
	}

	// Configurar formato desde variable de entorno
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.WithFormat(LogFormatFromString(format))
	}

	// Configurar source desde variable de entorno
	if source := os.Getenv("LOG_ADD_SOURCE"); source == "true" {
		config.WithSource(true)
	}

	return config
}

// getEnvOrDefault obtiene una variable de entorno o retorna un valor por defecto
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
