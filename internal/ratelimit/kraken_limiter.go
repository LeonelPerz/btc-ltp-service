package ratelimit

import (
	"time"

	"btc-ltp-service/internal/logger"
)

const (
	// Límites de Kraken API basados en documentación oficial
	// https://docs.kraken.com/rest/#section/Rate-Limits
	KrakenDefaultCapacity     = 15 // Máximo burst de peticiones
	KrakenDefaultRefillRate   = 1  // 1 token por segundo (60 por minuto)
	KrakenDefaultRefillPeriod = time.Second

	// Límites más conservadores para evitar baneos
	KrakenConservativeCapacity     = 10 // Burst más pequeño
	KrakenConservativeRefillRate   = 1  // 1 token cada 2 segundos (30 por minuto)
	KrakenConservativeRefillPeriod = 2 * time.Second
)

// KrakenRateLimiter encapsula el rate limiter específico para Kraken API
type KrakenRateLimiter struct {
	bucket    *TokenBucket
	isEnabled bool
	mode      string
}

// NewKrakenRateLimiter crea un rate limiter configurado para Kraken API
func NewKrakenRateLimiter(conservative bool) *KrakenRateLimiter {
	var capacity int64
	var refillRate int64
	var refillPeriod time.Duration
	var mode string

	if conservative {
		capacity = KrakenConservativeCapacity
		refillRate = KrakenConservativeRefillRate
		refillPeriod = KrakenConservativeRefillPeriod
		mode = "conservative"
	} else {
		capacity = KrakenDefaultCapacity
		refillRate = KrakenDefaultRefillRate
		refillPeriod = KrakenDefaultRefillPeriod
		mode = "default"
	}

	bucket := NewTokenBucket(capacity, refillRate, refillPeriod)

	limiter := &KrakenRateLimiter{
		bucket:    bucket,
		isEnabled: true,
		mode:      mode,
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"mode":          mode,
		"capacity":      capacity,
		"refill_rate":   refillRate,
		"refill_period": refillPeriod.String(),
		"enabled":       true,
	}).Info("Kraken rate limiter initialized")

	return limiter
}

// NewDefaultKrakenRateLimiter crea un rate limiter con configuración por defecto
func NewDefaultKrakenRateLimiter() *KrakenRateLimiter {
	return NewKrakenRateLimiter(false)
}

// NewConservativeKrakenRateLimiter crea un rate limiter con configuración conservadora
func NewConservativeKrakenRateLimiter() *KrakenRateLimiter {
	return NewKrakenRateLimiter(true)
}

// Allow intenta obtener permiso para hacer una petición
func (k *KrakenRateLimiter) Allow() bool {
	if !k.isEnabled {
		return true
	}
	return k.bucket.Allow()
}

// WaitForPermission espera hasta obtener permiso para hacer una petición
func (k *KrakenRateLimiter) WaitForPermission() {
	if !k.isEnabled {
		return
	}

	start := time.Now()
	k.bucket.WaitForToken()
	waitTime := time.Since(start)

	if waitTime > time.Millisecond*10 {
		logger.GetLogger().WithFields(map[string]interface{}{
			"wait_time": waitTime.String(),
			"mode":      k.mode,
		}).Debug("Rate limiter caused request delay")
	}
}

// Enable habilita o deshabilita el rate limiter
func (k *KrakenRateLimiter) Enable(enabled bool) {
	k.isEnabled = enabled

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"status": status,
		"mode":   k.mode,
	}).Info("Kraken rate limiter status changed")
}

// IsEnabled retorna si el rate limiter está habilitado
func (k *KrakenRateLimiter) IsEnabled() bool {
	return k.isEnabled
}

// GetStats retorna estadísticas del rate limiter
func (k *KrakenRateLimiter) GetStats() map[string]interface{} {
	stats := k.bucket.GetStats()
	stats["enabled"] = k.isEnabled
	stats["mode"] = k.mode
	return stats
}

// GetMode retorna el modo de configuración del rate limiter
func (k *KrakenRateLimiter) GetMode() string {
	return k.mode
}

// RateLimitConfig representa la configuración del rate limiter
// Define esta estructura aquí para evitar dependencia circular
type RateLimitConfig struct {
	Enabled      bool
	Conservative bool
	Capacity     int64
	RefillRate   int64
	RefillPeriod time.Duration
}

// NewKrakenRateLimiterFromConfig crea un rate limiter desde configuración personalizada
func NewKrakenRateLimiterFromConfig(config RateLimitConfig) *KrakenRateLimiter {
	var bucket *TokenBucket
	var mode string

	if config.Capacity > 0 && config.RefillRate > 0 && config.RefillPeriod > 0 {
		// Usar configuración personalizada
		bucket = NewTokenBucket(config.Capacity, config.RefillRate, config.RefillPeriod)
		mode = "custom"
	} else if config.Conservative {
		// Usar configuración conservadora por defecto
		bucket = NewTokenBucket(
			KrakenConservativeCapacity,
			KrakenConservativeRefillRate,
			KrakenConservativeRefillPeriod,
		)
		mode = "conservative"
	} else {
		// Usar configuración por defecto
		bucket = NewTokenBucket(
			KrakenDefaultCapacity,
			KrakenDefaultRefillRate,
			KrakenDefaultRefillPeriod,
		)
		mode = "default"
	}

	limiter := &KrakenRateLimiter{
		bucket:    bucket,
		isEnabled: config.Enabled,
		mode:      mode,
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"mode":          mode,
		"enabled":       config.Enabled,
		"capacity":      config.Capacity,
		"refill_rate":   config.RefillRate,
		"refill_period": config.RefillPeriod.String(),
	}).Info("Kraken rate limiter initialized from config")

	return limiter
}
