package logging

import (
	"io"
	"os"
)

// LoggerConfig contiene la configuración del sistema de logging
type LoggerConfig struct {
	Level       LogLevel  `json:"level" yaml:"level"`
	Format      LogFormat `json:"format" yaml:"format"`
	Output      io.Writer `json:"-" yaml:"-"`
	Service     string    `json:"service" yaml:"service"`
	Version     string    `json:"version" yaml:"version"`
	Environment string    `json:"environment" yaml:"environment"`
	AddSource   bool      `json:"add_source" yaml:"add_source"`
}

// LogFormat representa el formato de salida de los logs
type LogFormat string

const (
	FormatJSON LogFormat = "json"
	FormatText LogFormat = "text"
)

// DefaultConfig retorna la configuración por defecto
func DefaultConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:       LevelInfo,
		Format:      FormatJSON,
		Output:      os.Stdout,
		Service:     "btc-ltp-service",
		Version:     "1.0.0",
		Environment: "development",
		AddSource:   false,
	}
}

// NewConfig crea una nueva configuración con valores personalizados
func NewConfig(service, version, environment string) *LoggerConfig {
	config := DefaultConfig()
	config.Service = service
	config.Version = version
	config.Environment = environment
	return config
}

// WithLevel establece el nivel de log
func (c *LoggerConfig) WithLevel(level LogLevel) *LoggerConfig {
	c.Level = level
	return c
}

// WithFormat establece el formato de salida
func (c *LoggerConfig) WithFormat(format LogFormat) *LoggerConfig {
	c.Format = format
	return c
}

// WithOutput establece el writer de salida
func (c *LoggerConfig) WithOutput(output io.Writer) *LoggerConfig {
	c.Output = output
	return c
}

// WithSource habilita/deshabilita información de código fuente en logs
func (c *LoggerConfig) WithSource(addSource bool) *LoggerConfig {
	c.AddSource = addSource
	return c
}

// Validate valida la configuración
func (c *LoggerConfig) Validate() error {
	// Validar nivel de log
	validLevels := map[LogLevel]bool{
		LevelDebug: true,
		LevelInfo:  true,
		LevelWarn:  true,
		LevelError: true,
	}
	if !validLevels[c.Level] {
		return &ConfigError{Field: "level", Value: string(c.Level), Message: "invalid log level"}
	}

	// Validar formato
	validFormats := map[LogFormat]bool{
		FormatJSON: true,
		FormatText: true,
	}
	if !validFormats[c.Format] {
		return &ConfigError{Field: "format", Value: string(c.Format), Message: "invalid log format"}
	}

	// Validar output
	if c.Output == nil {
		return &ConfigError{Field: "output", Value: "nil", Message: "output writer cannot be nil"}
	}

	// Validar service name
	if c.Service == "" {
		return &ConfigError{Field: "service", Value: "", Message: "service name cannot be empty"}
	}

	return nil
}

// ConfigError representa un error de configuración
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error in field '" + e.Field + "' with value '" + e.Value + "': " + e.Message
}

// LogLevelFromString convierte un string a LogLevel
func LogLevelFromString(level string) LogLevel {
	switch level {
	case "DEBUG", "debug":
		return LevelDebug
	case "INFO", "info":
		return LevelInfo
	case "WARN", "warn", "WARNING", "warning":
		return LevelWarn
	case "ERROR", "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// LogFormatFromString convierte un string a LogFormat
func LogFormatFromString(format string) LogFormat {
	switch format {
	case "json", "JSON":
		return FormatJSON
	case "text", "TEXT":
		return FormatText
	default:
		return FormatJSON
	}
}
