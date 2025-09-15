package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	tests := []struct {
		name         string
		capacity     int64
		refillRate   int64
		refillPeriod time.Duration
		requests     int
		expected     []bool
	}{
		{
			name:         "básico - bucket lleno permite requests hasta capacidad",
			capacity:     3,
			refillRate:   1,
			refillPeriod: time.Second,
			requests:     5,
			expected:     []bool{true, true, true, false, false},
		},
		{
			name:         "capacidad 1 - solo permite 1 request",
			capacity:     1,
			refillRate:   1,
			refillPeriod: time.Second,
			requests:     3,
			expected:     []bool{true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewTokenBucket(tt.capacity, tt.refillRate, tt.refillPeriod)

			results := make([]bool, tt.requests)
			for i := 0; i < tt.requests; i++ {
				results[i] = tb.Allow()
			}

			for i, expected := range tt.expected {
				if i < len(results) && results[i] != expected {
					t.Errorf("Request %d: expected %v, got %v", i, expected, results[i])
				}
			}
		})
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	// Test con período de refill muy corto para poder probar
	tb := NewTokenBucket(2, 1, 10*time.Millisecond)

	// Consumir todos los tokens
	if !tb.Allow() {
		t.Error("First request should be allowed")
	}
	if !tb.Allow() {
		t.Error("Second request should be allowed")
	}
	if tb.Allow() {
		t.Error("Third request should be denied")
	}

	// Esperar a que se rellene
	time.Sleep(15 * time.Millisecond)

	// Ahora debería tener al menos 1 token
	if !tb.Allow() {
		t.Error("Request after refill should be allowed")
	}
}

func TestTokenBucket_GetTokens(t *testing.T) {
	capacity := int64(5)
	tb := NewTokenBucket(capacity, 1, time.Second)

	// Verificar capacidad inicial
	if tokens := tb.GetTokens(); tokens != capacity {
		t.Errorf("Expected %d tokens, got %d", capacity, tokens)
	}

	// Consumir algunos tokens
	tb.Allow()
	tb.Allow()

	if tokens := tb.GetTokens(); tokens != capacity-2 {
		t.Errorf("Expected %d tokens after consuming 2, got %d", capacity-2, tokens)
	}
}

func TestTokenBucket_GetCapacity(t *testing.T) {
	capacity := int64(10)
	tb := NewTokenBucket(capacity, 1, time.Second)

	if tb.GetCapacity() != capacity {
		t.Errorf("Expected capacity %d, got %d", capacity, tb.GetCapacity())
	}
}

func TestTokenBucket_GetStats(t *testing.T) {
	tb := NewTokenBucket(5, 2, time.Minute)

	stats := tb.GetStats()

	if stats["capacity"] != int64(5) {
		t.Errorf("Expected capacity 5, got %v", stats["capacity"])
	}
	if stats["refill_rate"] != int64(2) {
		t.Errorf("Expected refill_rate 2, got %v", stats["refill_rate"])
	}
	if stats["refill_period"] != "1m0s" {
		t.Errorf("Expected refill_period '1m0s', got %v", stats["refill_period"])
	}
}

func TestTokenBucket_WaitForToken(t *testing.T) {
	// Test con período de refill muy corto
	tb := NewTokenBucket(1, 1, 20*time.Millisecond)

	// Consumir el token disponible
	if !tb.Allow() {
		t.Error("First request should be allowed")
	}

	// WaitForToken debería esperar hasta que haya un token disponible
	start := time.Now()
	tb.WaitForToken()
	elapsed := time.Since(start)

	if elapsed < 15*time.Millisecond {
		t.Errorf("WaitForToken should have waited at least 15ms, waited %v", elapsed)
	}

	// Después de WaitForToken, no debería haber tokens disponibles (fue consumido)
	if tb.Allow() {
		t.Error("Token should have been consumed by WaitForToken")
	}
}

// Benchmark para verificar performance
func BenchmarkTokenBucket_Allow(b *testing.B) {
	tb := NewTokenBucket(1000, 100, time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow()
	}
}
