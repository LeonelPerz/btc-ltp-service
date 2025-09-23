package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Validator valida la configuración cargada
type Validator struct{}

// NewValidator crea una nueva instancia del validador
func NewValidator() *Validator {
	return &Validator{}
}

// Validate valida toda la configuración
func (v *Validator) Validate(config *Config) error {
	// Validar que los valores no sean defaults inesperados (detecta parsing errors)
	if err := v.validateConfigIntegrity(config); err != nil {
		return fmt.Errorf("config parsing validation failed: %w", err)
	}

	if err := v.validateServer(config.Server); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}

	if err := v.validateCache(config.Cache); err != nil {
		return fmt.Errorf("cache config validation failed: %w", err)
	}

	if err := v.validateExchange(config.Exchange); err != nil {
		return fmt.Errorf("exchange config validation failed: %w", err)
	}

	if err := v.validateRateLimit(config.RateLimit); err != nil {
		return fmt.Errorf("rate limit config validation failed: %w", err)
	}

	if err := v.validateLogging(config.Logging); err != nil {
		return fmt.Errorf("logging config validation failed: %w", err)
	}

	if err := v.validateBusiness(config.Business); err != nil {
		return fmt.Errorf("business config validation failed: %w", err)
	}

	return nil
}

// validateServer valida la configuración del servidor
func (v *Validator) validateServer(config ServerConfig) error {
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 1-65535", config.Port)
	}

	if config.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout must be positive, got: %v", config.ShutdownTimeout)
	}

	if config.ShutdownTimeout > 5*time.Minute {
		return fmt.Errorf("shutdown_timeout too long: %v, max 5 minutes", config.ShutdownTimeout)
	}

	return nil
}

// validateCache valida la configuración del cache
func (v *Validator) validateCache(config CacheConfig) error {
	validBackends := []string{"memory", "redis"}
	if !contains(validBackends, config.Backend) {
		return fmt.Errorf("invalid cache backend: %s, must be one of: %v", config.Backend, validBackends)
	}

	// Validación mejorada de TTL para detectar casos edge
	if err := v.validateTTL(config.TTL); err != nil {
		return fmt.Errorf("cache TTL validation failed: %w", err)
	}

	// Validar Redis config si se usa Redis
	if config.Backend == "redis" {
		if err := v.validateRedis(config.Redis); err != nil {
			return err
		}
	}

	return nil
}

// validateRedis valida la configuración de Redis
func (v *Validator) validateRedis(config RedisConfig) error {
	if config.Addr == "" {
		return fmt.Errorf("redis addr cannot be empty")
	}

	// Validar formato de dirección
	if !strings.Contains(config.Addr, ":") {
		return fmt.Errorf("invalid redis addr format: %s, expected host:port", config.Addr)
	}

	if config.DB < 0 || config.DB > 15 {
		return fmt.Errorf("invalid redis DB: %d, must be between 0-15", config.DB)
	}

	return nil
}

// validateExchange valida la configuración de exchanges
func (v *Validator) validateExchange(config ExchangeConfig) error {
	return v.validateKraken(config.Kraken)
}

// validateKraken valida la configuración específica de Kraken
func (v *Validator) validateKraken(config KrakenConfig) error {
	// Validar URLs
	if err := v.validateURL(config.RestURL, "kraken rest_url"); err != nil {
		return err
	}

	if err := v.validateWebSocketURL(config.WebSocketURL, "kraken websocket_url"); err != nil {
		return err
	}

	// Validar timeouts
	if config.Timeout <= 0 {
		return fmt.Errorf("kraken timeout must be positive, got: %v", config.Timeout)
	}

	if config.RequestTimeout <= 0 {
		return fmt.Errorf("kraken request_timeout must be positive, got: %v", config.RequestTimeout)
	}

	if config.FallbackTimeout <= 0 {
		return fmt.Errorf("kraken fallback_timeout must be positive, got: %v", config.FallbackTimeout)
	}

	if config.RequestTimeout >= config.Timeout {
		return fmt.Errorf("kraken request_timeout (%v) should be less than timeout (%v)", config.RequestTimeout, config.Timeout)
	}

	// Validar retries
	if config.MaxRetries < 1 || config.MaxRetries > 10 {
		return fmt.Errorf("kraken max_retries must be between 1-10, got: %d", config.MaxRetries)
	}

	return nil
}

// validateRateLimit valida la configuración de rate limiting
func (v *Validator) validateRateLimit(config RateLimitConfig) error {
	if config.Enabled {
		if config.Capacity <= 0 {
			return fmt.Errorf("rate_limit capacity must be positive when enabled, got: %d", config.Capacity)
		}

		if config.RefillRate <= 0 {
			return fmt.Errorf("rate_limit refill_rate must be positive when enabled, got: %d", config.RefillRate)
		}

		if config.Capacity > 10000 {
			return fmt.Errorf("rate_limit capacity too high: %d, max 10000", config.Capacity)
		}

		if config.RefillRate > 1000 {
			return fmt.Errorf("rate_limit refill_rate too high: %d, max 1000", config.RefillRate)
		}
	}

	return nil
}

// validateLogging valida la configuración de logging
func (v *Validator) validateLogging(config LoggingConfig) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, strings.ToLower(config.Level)) {
		return fmt.Errorf("invalid log level: %s, must be one of: %v", config.Level, validLevels)
	}

	validFormats := []string{"json", "text"}
	if !contains(validFormats, strings.ToLower(config.Format)) {
		return fmt.Errorf("invalid log format: %s, must be one of: %v", config.Format, validFormats)
	}

	return nil
}

// validateBusiness valida la configuración de negocio
func (v *Validator) validateBusiness(config BusinessConfig) error {
	if len(config.SupportedPairs) == 0 {
		return fmt.Errorf("supported_pairs cannot be empty")
	}

	// Validación robusta de pares con lista de pares conocidos
	if err := v.validateTradingPairs(config.SupportedPairs); err != nil {
		return fmt.Errorf("trading pairs validation failed: %w", err)
	}

	if config.CachePrefix == "" {
		return fmt.Errorf("cache_prefix cannot be empty")
	}

	return nil
}

// validateURL valida que una URL sea válida para HTTP/HTTPS
func (v *Validator) validateURL(rawURL, fieldName string) error {
	if rawURL == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid %s: %s, error: %v", fieldName, rawURL, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid %s scheme: %s, must be http or https", fieldName, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s must have a host", fieldName)
	}

	return nil
}

// validateWebSocketURL valida que una URL sea válida para WebSocket
func (v *Validator) validateWebSocketURL(rawURL, fieldName string) error {
	if rawURL == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid %s: %s, error: %v", fieldName, rawURL, err)
	}

	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return fmt.Errorf("invalid %s scheme: %s, must be ws or wss", fieldName, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("%s must have a host", fieldName)
	}

	return nil
}

// contains verifica si un slice contiene un elemento
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// validateTTL valida TTL con casos edge específicos para fail-fast
func (v *Validator) validateTTL(ttl time.Duration) error {
	// TTL debe ser positivo
	if ttl <= 0 {
		return fmt.Errorf("TTL must be positive, got: %v", ttl)
	}

	// Casos edge específicos que causan problemas
	if ttl < 100*time.Millisecond {
		return fmt.Errorf("TTL too short: %v, minimum 100ms (causes excessive cache churn)", ttl)
	}

	if ttl > 24*time.Hour {
		return fmt.Errorf("TTL too long: %v, maximum 24h (stale data risk)", ttl)
	}

	// Advertencia para TTL muy corto o muy largo (sub-optimal pero no crítico)
	if ttl < 1*time.Second {
		return fmt.Errorf("TTL potentially inefficient: %v, recommended minimum 1s for production", ttl)
	}

	if ttl > 1*time.Hour {
		return fmt.Errorf("TTL potentially stale: %v, recommended maximum 1h for financial data", ttl)
	}

	return nil
}

// validateTradingPairs valida pares contra lista conocida de Kraken
func (v *Validator) validateTradingPairs(pairs []string) error {
	if len(pairs) == 0 {
		return fmt.Errorf("trading pairs list cannot be empty")
	}

	// Lista de pares conocidos en Kraken (actualizada frecuentemente)
	knownPairs := v.getKnownKrakenPairs()
	unknownPairs := make([]string, 0)
	invalidFormatPairs := make([]string, 0)

	for _, pair := range pairs {
		pair = strings.TrimSpace(strings.ToUpper(pair))

		// Validar formato básico
		if !strings.Contains(pair, "/") {
			invalidFormatPairs = append(invalidFormatPairs, pair)
			continue
		}

		parts := strings.Split(pair, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			invalidFormatPairs = append(invalidFormatPairs, pair)
			continue
		}

		// Validar contra pares conocidos
		if !v.isKnownPair(pair, knownPairs) {
			unknownPairs = append(unknownPairs, pair)
		}
	}

	// Reportar errores específicos para debugging rápido
	if len(invalidFormatPairs) > 0 {
		return fmt.Errorf("invalid pair format: %v, expected BASE/QUOTE (e.g., BTC/USD)", invalidFormatPairs)
	}

	if len(unknownPairs) > 0 {
		return fmt.Errorf("unknown trading pairs: %v, supported pairs: %v", unknownPairs, v.getSampleKnownPairs())
	}

	return nil
}

// getKnownKrakenPairs retorna lista de pares conocidos en Kraken
func (v *Validator) getKnownKrakenPairs() map[string]bool {
	// Lista basada en pares populares de Kraken (2024)
	// En producción, esto podría venir de una API o archivo de configuración
	return map[string]bool{
		"BTC/USD":  true,
		"BTC/EUR":  true,
		"BTC/CHF":  true,
		"BTC/GBP":  true,
		"ETH/USD":  true,
		"ETH/EUR":  true,
		"ETH/BTC":  true,
		"LTC/USD":  true,
		"LTC/EUR":  true,
		"LTC/BTC":  true,
		"XRP/USD":  true,
		"XRP/EUR":  true,
		"XRP/BTC":  true,
		"ADA/USD":  true,
		"ADA/EUR":  true,
		"DOT/USD":  true,
		"DOT/EUR":  true,
		"LINK/USD": true,
		"LINK/EUR": true,
		"UNI/USD":  true,
		"UNI/EUR":  true,
		"SOL/USD":  true,
		"SOL/EUR":  true,
	}
}

// isKnownPair verifica si un par está en la lista conocida
func (v *Validator) isKnownPair(pair string, knownPairs map[string]bool) bool {
	return knownPairs[strings.ToUpper(pair)]
}

// getSampleKnownPairs retorna muestra de pares para mensajes de error
func (v *Validator) getSampleKnownPairs() []string {
	return []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD", "BTC/EUR", "ETH/EUR"}
}

// validateConfigIntegrity verifica que la configuración fue parseada correctamente
// Detecta cuando Viper usa defaults por errores de parsing silenciosos
func (v *Validator) validateConfigIntegrity(config *Config) error {
	// Para detectar parsing errors, validamos que valores críticos no sean exactamente
	// iguales a los defaults cuando sabemos que deberían ser diferentes

	// Si TTL es exactamente 30s (default) y backend es memory, puede indicar parsing error
	// Esta validación es más heurística - en producción podríamos usar Viper directamente

	// Validaciones de integridad específicas:

	// 1. TTL de 0 indica parsing error crítico
	if config.Cache.TTL == 0 {
		return fmt.Errorf("cache TTL parsed as 0, likely due to invalid duration format in config (e.g., 'abc', 'invalid-string')")
	}

	// 2. Puerto 0 indica parsing error crítico
	if config.Server.Port == 0 {
		return fmt.Errorf("server port parsed as 0, likely due to invalid port format in config")
	}

	// 3. Timeout de 0 indica parsing error
	if config.Exchange.Kraken.Timeout == 0 {
		return fmt.Errorf("kraken timeout parsed as 0, likely due to invalid duration format in config")
	}

	// 4. Verificar que strings críticos no estén vacíos cuando no deberían
	if config.Business.CachePrefix == "" {
		return fmt.Errorf("cache prefix is empty, check configuration syntax")
	}

	return nil
}
