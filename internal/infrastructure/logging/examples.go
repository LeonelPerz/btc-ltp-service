package logging

import (
	"context"
	"errors"
	"time"
)

/*
Este archivo contiene ejemplos de cómo usar el nuevo sistema de logging estructurado mejorado.

VENTAJAS DEL NUEVO SISTEMA:

1. **Separación clara de responsabilidades**: Cada tipo de log tiene su propio logger especializado
2. **Campos estandarizados**: Uso de constantes para campos comunes
3. **Builder pattern**: Construcción fácil de campos complejos
4. **Configuración flexible**: Soporte para diferentes entornos y formatos
5. **Compatibilidad**: Mantiene la API existente para facilitar la migración
6. **Tipado fuerte**: Interfaces claras para cada tipo de logger

EJEMPLOS DE USO:
*/

// ExampleBasicLogging demuestra el uso básico del sistema de logging
func ExampleBasicLogging() {
	ctx := context.Background()

	// Logging básico con el logger global (compatible con código existente)
	Info(ctx, "Aplicación iniciada", Fields{
		"version": "1.0.0",
		"port":    8080,
	})

	// Usando el builder para campos más complejos
	fields := NewFieldBuilder().
		WithCustomField("user_id", "12345").
		WithCustomField("action", "login").
		WithDuration(time.Millisecond * 150).
		Build()

	Info(ctx, "Usuario autenticado", fields)
}

// ExampleHTTPLogging demuestra el uso del logger HTTP especializado
func ExampleHTTPLogging() {
	ctx := context.Background()
	httpLogger := HTTP()

	// Log cuando se recibe un request
	httpLogger.RequestReceived(ctx, "GET", "/api/btc/price", "curl/7.68.0", "192.168.1.1")

	// Log cuando se completa un request exitoso
	httpLogger.RequestCompleted(ctx, "GET", "/api/btc/price", 200, 45.5)

	// Log cuando falla un request
	err := errors.New("service unavailable")
	httpLogger.RequestFailed(ctx, "GET", "/api/btc/price", 503, err, 1200.0)
}

// ExampleExternalAPILogging demuestra el logging de APIs externas
func ExampleExternalAPILogging() {
	ctx := context.Background()
	apiLogger := ExternalAPI()

	// Log cuando se inicia un request externo
	apiLogger.RequestStarted(ctx, "kraken", "/0/public/Ticker", "GET")

	// Log cuando se completa exitosamente
	apiLogger.RequestCompleted(ctx, "kraken", "/0/public/Ticker", 200, 156.7)

	// Log cuando falla
	err := errors.New("connection timeout")
	apiLogger.RequestFailed(ctx, "kraken", "/0/public/Ticker", 0, err, 5000.0)
}

// ExampleCacheLogging demuestra el logging de operaciones de cache
func ExampleCacheLogging() {
	ctx := context.Background()
	cacheLogger := Cache()

	// Cache hit
	cacheLogger.Hit(ctx, "btc_usd_price", "GET")

	// Cache miss
	cacheLogger.Miss(ctx, "eth_usd_price", "GET")

	// Set en cache
	cacheLogger.Set(ctx, "btc_usd_price", 60.0) // TTL en segundos

	// Delete del cache
	cacheLogger.Delete(ctx, "old_price_key")

	// Error en operación de cache
	err := errors.New("redis connection failed")
	cacheLogger.CacheError(ctx, "GET", "btc_usd_price", err)
}

// ExampleBusinessLogging demuestra el logging de lógica de negocio
func ExampleBusinessLogging() {
	ctx := context.Background()
	businessLogger := Business()

	// Log cuando se solicita un precio
	businessLogger.PriceRequested(ctx, "BTCUSD", "kraken")

	// Log cuando se sirve un precio
	businessLogger.PriceServed(ctx, "BTCUSD", 45250.50, "kraken", true)

	// Log cuando falla la actualización de precio
	err := errors.New("exchange rate limit exceeded")
	businessLogger.PriceUpdateFailed(ctx, "BTCUSD", err)

	// Log cuando falla la validación
	businessLogger.ValidationFailed(ctx, "INVALID_PAIR", "unsupported trading pair")
}

// ExampleSecurityLogging demuestra el logging de eventos de seguridad
func ExampleSecurityLogging() {
	ctx := context.Background()
	securityLogger := Security()

	// Rate limit excedido
	securityLogger.RateLimitExceeded(ctx, "192.168.1.100", "/api/btc/price")

	// Request inválido
	securityLogger.InvalidRequest(ctx, "192.168.1.100", "malformed JSON body")

	// Actividad sospechosa
	securityLogger.SuspiciousActivity(ctx, "10.0.0.1", "SQL injection attempt")
}

// ExampleAdvancedFieldBuilder demuestra el uso avanzado del field builder
func ExampleAdvancedFieldBuilder() {
	ctx := context.Background()

	// Ejemplo complejo con múltiples tipos de información
	fields := NewFieldBuilder().
		WithHTTPInfo("POST", "/api/orders", 201).
		WithUserAgent("MyApp/1.0").
		WithRemoteIP("203.0.113.1").
		WithDuration(time.Millisecond*234).
		WithCustomField("order_id", "ORD-123456").
		WithCustomField("amount", 1.5).
		WithCustomField("currency", "BTC").
		Build()

	Info(ctx, "Orden creada exitosamente", fields)

	// Ejemplo con información de API externa
	externalFields := NewFieldBuilder().
		WithExternalAPI("binance", "/api/v3/ticker/price", 200, time.Millisecond*89).
		WithCustomField("symbol", "BTCUSDT").
		Build()

	Info(ctx, "Precio obtenido de exchange externo", externalFields)

	// Ejemplo con error
	err := errors.New("database connection failed")
	errorFields := NewFieldBuilder().
		WithError(err).
		WithCustomField("operation", "user_create").
		WithCustomField("user_id", "usr_789").
		WithDuration(time.Millisecond * 1500).
		Build()

	ErrorWithError(ctx, "Falló la creación del usuario", err, errorFields)
}

// ExampleContextualLogging demuestra cómo usar el contexto para tracking
func ExampleContextualLogging() {
	// Crear contexto con información de request
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req_abc123")
	ctx = WithStartTime(ctx, time.Now())
	ctx = WithUserAgent(ctx, "Mozilla/5.0...")
	ctx = WithRemoteIP(ctx, "192.168.1.50")

	// Los logs automáticamente incluirán la información del contexto
	Info(ctx, "Procesando request", Fields{
		"endpoint": "/api/btc/price",
		"method":   "GET",
	})

	// Simular algo de trabajo...
	time.Sleep(time.Millisecond * 100)

	// Este log incluirá automáticamente la duración calculada desde start_time
	Info(ctx, "Request completado", Fields{
		"status": "success",
		"price":  45250.75,
	})
}

// ExampleCustomLoggerConfiguration demuestra configuración personalizada
func ExampleCustomLoggerConfiguration() {
	// Configuración para desarrollo
	devConfig := NewDevelopmentConfig("my-btc-service").
		WithLevel(LevelDebug).
		WithSource(true)

	devFactory, _ := NewLoggerFactory(devConfig)
	devLogger := devFactory.GetBaseLogger()

	ctx := context.Background()
	devLogger.Debug(ctx, "Debug info para desarrollo", Fields{
		"debug_data": map[string]interface{}{
			"memory_usage": "45MB",
			"goroutines":   12,
		},
	})

	// Configuración para producción
	prodConfig := NewProductionConfig("btc-ltp-service", "2.1.0").
		WithLevel(LevelInfo).
		WithFormat(FormatJSON)

	prodFactory, _ := NewLoggerFactory(prodConfig)
	prodLoggers := prodFactory.GetLoggerSet()

	// Usar loggers especializados en producción
	prodLoggers.Business.PriceServed(ctx, "BTCUSD", 45250.50, "kraken", true)
	prodLoggers.Security.RateLimitExceeded(ctx, "10.0.0.1", "/api/btc/price")
}

/*
MIGRACIÓN DESDE EL SISTEMA ANTERIOR:

1. El código existente seguirá funcionando sin cambios:
   logging.Info(ctx, "message", fields) // ✅ Funciona igual

2. Gradualmente puedes migrar a loggers especializados:
   logging.HTTP().RequestCompleted(ctx, "GET", "/api", 200, 45.5)

3. Usa el FieldBuilder para logs más complejos:
   fields := logging.NewFieldBuilder().
       WithHTTPInfo("GET", "/api", 200).
       WithDuration(time.Millisecond * 45).
       Build()

4. Configura el sistema al inicio de la aplicación:
   config := logging.NewProductionConfig("my-service", "1.0.0")
   logging.InitializeGlobalLoggers(config)

FORMATO DE SALIDA MEJORADO:

JSON Format (producción):
{
  "timestamp": "2023-12-07T10:30:45Z",
  "level": "INFO",
  "message": "HTTP request completed",
  "request_id": "req_123",
  "service": "btc-ltp-service",
  "version": "1.0.0",
  "domain": "http",
  "fields": {
    "http_method": "GET",
    "http_path": "/api/btc/price",
    "http_status_code": 200,
    "duration_ms": 45.5
  }
}

Text Format (desarrollo):
2023-12-07T10:30:45Z [INFO] req:req_123 domain:http HTTP request completed fields={"http_method":"GET","http_path":"/api/btc/price","http_status_code":200,"duration_ms":45.5}
*/
