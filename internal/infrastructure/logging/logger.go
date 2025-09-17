package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// StructuredLogger implementa la interfaz Logger con logging estructurado
type StructuredLogger struct {
	config *LoggerConfig
	logger *log.Logger
}

// LogEntry representa una entrada de log estructurada
type LogEntry struct {
	Timestamp   string   `json:"timestamp"`
	Level       LogLevel `json:"level"`
	Message     string   `json:"message"`
	RequestID   string   `json:"request_id,omitempty"`
	Service     string   `json:"service"`
	Version     string   `json:"version,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Source      string   `json:"source,omitempty"`
	Fields      Fields   `json:"fields,omitempty"`
}

// NewStructuredLogger crea un nuevo logger estructurado
func NewStructuredLogger(config *LoggerConfig) (*StructuredLogger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger config: %w", err)
	}

	return &StructuredLogger{
		config: config,
		logger: log.New(config.Output, "", 0),
	}, nil
}

// shouldLog verifica si un mensaje debe ser registrado basado en el nivel
func (sl *StructuredLogger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
	}
	return levels[level] >= levels[sl.config.Level]
}

// log escribe una entrada de log estructurada
func (sl *StructuredLogger) log(ctx context.Context, level LogLevel, message string, fields Fields) {
	if !sl.shouldLog(level) {
		return
	}

	entry := sl.createLogEntry(ctx, level, message, fields)

	// Formatear según configuración
	var output string
	switch sl.config.Format {
	case FormatJSON:
		output = sl.formatJSON(entry)
	case FormatText:
		output = sl.formatText(entry)
	default:
		output = sl.formatJSON(entry)
	}

	sl.logger.Println(output)
}

// createLogEntry crea una entrada de log con toda la información necesaria
func (sl *StructuredLogger) createLogEntry(ctx context.Context, level LogLevel, message string, fields Fields) *LogEntry {
	entry := &LogEntry{
		Timestamp:   time.Now().Format(time.RFC3339),
		Level:       level,
		Message:     message,
		Service:     sl.config.Service,
		Version:     sl.config.Version,
		Environment: sl.config.Environment,
		RequestID:   GetRequestID(ctx),
		Fields:      fields,
	}

	// Agregar duración si hay tiempo de inicio en el contexto
	if startTime := GetStartTime(ctx); !startTime.IsZero() {
		if entry.Fields == nil {
			entry.Fields = make(Fields)
		}
		entry.Fields[FieldDuration] = float64(time.Since(startTime).Nanoseconds()) / 1e6
	}

	// Agregar información de código fuente si está habilitada
	if sl.config.AddSource {
		if source := sl.getSource(); source != "" {
			entry.Source = source
		}
	}

	return entry
}

// formatJSON formatea la entrada como JSON
func (sl *StructuredLogger) formatJSON(entry *LogEntry) string {
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback a formato simple si JSON falla
		return fmt.Sprintf("[%s] %s - %s", entry.Level, entry.RequestID, entry.Message)
	}
	return string(jsonData)
}

// formatText formatea la entrada como texto legible
func (sl *StructuredLogger) formatText(entry *LogEntry) string {
	var parts []string
	parts = append(parts, entry.Timestamp)
	parts = append(parts, fmt.Sprintf("[%s]", entry.Level))

	if entry.RequestID != "" {
		parts = append(parts, fmt.Sprintf("req:%s", entry.RequestID))
	}

	if entry.Domain != "" {
		parts = append(parts, fmt.Sprintf("domain:%s", entry.Domain))
	}

	parts = append(parts, entry.Message)

	result := strings.Join(parts, " ")

	// Agregar campos si existen
	if len(entry.Fields) > 0 {
		if fieldsJson, err := json.Marshal(entry.Fields); err == nil {
			result += fmt.Sprintf(" fields=%s", string(fieldsJson))
		}
	}

	return result
}

// getSource obtiene información del código fuente que llamó al logger
func (sl *StructuredLogger) getSource() string {
	// Skip: runtime.Callers, getSource, log method, public method
	const skip = 4
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	name := fn.Name()
	// Simplificar el nombre eliminando prefijos de paquete largos
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(ctx context.Context, message string, fields Fields) {
	sl.log(ctx, LevelDebug, message, fields)
}

// Info logs an info message
func (sl *StructuredLogger) Info(ctx context.Context, message string, fields Fields) {
	sl.log(ctx, LevelInfo, message, fields)
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(ctx context.Context, message string, fields Fields) {
	sl.log(ctx, LevelWarn, message, fields)
}

// Error logs an error message
func (sl *StructuredLogger) Error(ctx context.Context, message string, fields Fields) {
	sl.log(ctx, LevelError, message, fields)
}

// InfoWithError logs an info message with error details
func (sl *StructuredLogger) InfoWithError(ctx context.Context, message string, err error, fields Fields) {
	enrichedFields := sl.enrichWithError(fields, err)
	sl.log(ctx, LevelInfo, message, enrichedFields)
}

// WarnWithError logs a warning message with error details
func (sl *StructuredLogger) WarnWithError(ctx context.Context, message string, err error, fields Fields) {
	enrichedFields := sl.enrichWithError(fields, err)
	sl.log(ctx, LevelWarn, message, enrichedFields)
}

// ErrorWithError logs an error message with error details
func (sl *StructuredLogger) ErrorWithError(ctx context.Context, message string, err error, fields Fields) {
	enrichedFields := sl.enrichWithError(fields, err)
	sl.log(ctx, LevelError, message, enrichedFields)
}

// enrichWithError enriquece los campos con información del error
func (sl *StructuredLogger) enrichWithError(fields Fields, err error) Fields {
	if err == nil {
		return fields
	}

	if fields == nil {
		fields = make(Fields)
	}

	fields[FieldError] = err.Error()
	fields[FieldErrorType] = getErrorType(err)
	return fields
}

// SetLevel establece el nivel de logging
func (sl *StructuredLogger) SetLevel(level LogLevel) {
	sl.config.Level = level
}

// GetLevel retorna el nivel actual de logging
func (sl *StructuredLogger) GetLevel() LogLevel {
	return sl.config.Level
}

// GetConfig retorna la configuración actual
func (sl *StructuredLogger) GetConfig() *LoggerConfig {
	return sl.config
}

// Clone crea una copia del logger con configuración modificable
func (sl *StructuredLogger) Clone() *StructuredLogger {
	configCopy := *sl.config
	return &StructuredLogger{
		config: &configCopy,
		logger: log.New(configCopy.Output, "", 0),
	}
}
