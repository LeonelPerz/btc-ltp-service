package ratelimit

import (
	"sync"
	"time"

	"btc-ltp-service/internal/logger"
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	capacity     int64         // Máximo número de tokens en el bucket
	tokens       int64         // Tokens actuales en el bucket
	refillRate   int64         // Tokens agregados por refill interval
	refillPeriod time.Duration // Intervalo entre refills
	lastRefill   time.Time     // Última vez que se rellenaron los tokens
	mu           sync.Mutex    // Protege el estado del bucket
}

// NewTokenBucket crea un nuevo token bucket rate limiter
func NewTokenBucket(capacity int64, refillRate int64, refillPeriod time.Duration) *TokenBucket {
	tb := &TokenBucket{
		capacity:     capacity,
		tokens:       capacity, // Empezar con el bucket lleno
		refillRate:   refillRate,
		refillPeriod: refillPeriod,
		lastRefill:   time.Now(),
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"capacity":      capacity,
		"refill_rate":   refillRate,
		"refill_period": refillPeriod.String(),
	}).Info("Token bucket rate limiter initialized")

	return tb
}

// Allow intenta consumir un token del bucket
// Retorna true si el token fue otorgado, false si no hay tokens disponibles
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Rellenar tokens basado en el tiempo transcurrido
	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		logger.GetLogger().WithFields(map[string]interface{}{
			"remaining_tokens": tb.tokens,
			"capacity":         tb.capacity,
		}).Debug("Token granted by rate limiter")
		return true
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"remaining_tokens": tb.tokens,
		"capacity":         tb.capacity,
	}).Debug("Token denied by rate limiter - bucket empty")
	return false
}

// WaitForToken espera hasta que un token esté disponible
// Bloquea hasta que se pueda otorgar un token
func (tb *TokenBucket) WaitForToken() {
	for {
		if tb.Allow() {
			return
		}
		// Esperar un breve periodo antes de intentar de nuevo
		time.Sleep(time.Millisecond * 100)
	}
}

// refill agrega tokens al bucket basado en el tiempo transcurrido
// DEBE ser llamado con el mutex locked
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	if elapsed < tb.refillPeriod {
		return // No es tiempo de rellenar aún
	}

	// Calcular cuántos intervalos han pasado
	intervals := elapsed / tb.refillPeriod
	tokensToAdd := int64(intervals) * tb.refillRate

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity // No exceder la capacidad
		}
		tb.lastRefill = now

		logger.GetLogger().WithFields(map[string]interface{}{
			"tokens_added":   tokensToAdd,
			"current_tokens": tb.tokens,
			"capacity":       tb.capacity,
			"elapsed":        elapsed.String(),
		}).Debug("Token bucket refilled")
	}
}

// GetTokens retorna el número actual de tokens disponibles
func (tb *TokenBucket) GetTokens() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}

// GetCapacity retorna la capacidad máxima del bucket
func (tb *TokenBucket) GetCapacity() int64 {
	return tb.capacity
}

// GetStats retorna estadísticas del rate limiter
func (tb *TokenBucket) GetStats() map[string]interface{} {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()

	return map[string]interface{}{
		"current_tokens": tb.tokens,
		"capacity":       tb.capacity,
		"refill_rate":    tb.refillRate,
		"refill_period":  tb.refillPeriod.String(),
		"utilization":    float64(tb.capacity-tb.tokens) / float64(tb.capacity),
		"last_refill":    tb.lastRefill,
	}
}
