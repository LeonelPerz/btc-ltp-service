package ratelimit

import (
	"testing"
	"time"
)

func TestNewKrakenRateLimiter(t *testing.T) {
	tests := []struct {
		name         string
		conservative bool
		expectedMode string
	}{
		{
			name:         "modo conservador",
			conservative: true,
			expectedMode: "conservative",
		},
		{
			name:         "modo por defecto",
			conservative: false,
			expectedMode: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewKrakenRateLimiter(tt.conservative)

			if limiter.GetMode() != tt.expectedMode {
				t.Errorf("Expected mode %s, got %s", tt.expectedMode, limiter.GetMode())
			}

			if !limiter.IsEnabled() {
				t.Error("Rate limiter should be enabled by default")
			}
		})
	}
}

func TestNewDefaultKrakenRateLimiter(t *testing.T) {
	limiter := NewDefaultKrakenRateLimiter()

	if limiter.GetMode() != "default" {
		t.Errorf("Expected default mode, got %s", limiter.GetMode())
	}

	if !limiter.IsEnabled() {
		t.Error("Rate limiter should be enabled")
	}
}

func TestNewConservativeKrakenRateLimiter(t *testing.T) {
	limiter := NewConservativeKrakenRateLimiter()

	if limiter.GetMode() != "conservative" {
		t.Errorf("Expected conservative mode, got %s", limiter.GetMode())
	}

	if !limiter.IsEnabled() {
		t.Error("Rate limiter should be enabled")
	}
}

func TestKrakenRateLimiter_Enable(t *testing.T) {
	limiter := NewDefaultKrakenRateLimiter()

	// Inicialmente habilitado
	if !limiter.IsEnabled() {
		t.Error("Rate limiter should be enabled initially")
	}

	// Deshabilitar
	limiter.Enable(false)
	if limiter.IsEnabled() {
		t.Error("Rate limiter should be disabled after Enable(false)")
	}

	// Habilitar nuevamente
	limiter.Enable(true)
	if !limiter.IsEnabled() {
		t.Error("Rate limiter should be enabled after Enable(true)")
	}
}

func TestKrakenRateLimiter_Allow_WhenDisabled(t *testing.T) {
	limiter := NewDefaultKrakenRateLimiter()
	limiter.Enable(false)

	// Cuando está deshabilitado, siempre debería permitir
	for i := 0; i < 100; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should be allowed when rate limiter is disabled", i)
		}
	}
}

func TestKrakenRateLimiter_Allow_WhenEnabled(t *testing.T) {
	// Usar configuración conservadora para test predecible
	limiter := NewConservativeKrakenRateLimiter()

	// Debería permitir hasta la capacidad
	allowedCount := 0
	for i := 0; i < 20; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	// Debería haber permitido exactamente la capacidad conservadora (10)
	if allowedCount != KrakenConservativeCapacity {
		t.Errorf("Expected %d allowed requests, got %d", KrakenConservativeCapacity, allowedCount)
	}
}

func TestKrakenRateLimiter_WaitForPermission_WhenDisabled(t *testing.T) {
	limiter := NewDefaultKrakenRateLimiter()
	limiter.Enable(false)

	start := time.Now()
	limiter.WaitForPermission()
	elapsed := time.Since(start)

	// Cuando está deshabilitado, no debería esperar
	if elapsed > time.Millisecond {
		t.Errorf("WaitForPermission should not wait when disabled, waited %v", elapsed)
	}
}

func TestKrakenRateLimiter_GetStats(t *testing.T) {
	limiter := NewDefaultKrakenRateLimiter()

	stats := limiter.GetStats()

	// Verificar campos esperados
	if _, exists := stats["enabled"]; !exists {
		t.Error("Stats should include 'enabled' field")
	}
	if _, exists := stats["mode"]; !exists {
		t.Error("Stats should include 'mode' field")
	}
	if _, exists := stats["current_tokens"]; !exists {
		t.Error("Stats should include 'current_tokens' field")
	}
	if _, exists := stats["capacity"]; !exists {
		t.Error("Stats should include 'capacity' field")
	}

	if stats["enabled"] != true {
		t.Error("Stats should show enabled=true")
	}
	if stats["mode"] != "default" {
		t.Errorf("Stats should show mode='default', got %v", stats["mode"])
	}
}

func TestNewKrakenRateLimiterFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       RateLimitConfig
		expectedMode string
	}{
		{
			name: "configuración personalizada",
			config: RateLimitConfig{
				Enabled:      true,
				Conservative: false,
				Capacity:     5,
				RefillRate:   2,
				RefillPeriod: time.Second,
			},
			expectedMode: "custom",
		},
		{
			name: "configuración conservadora por defecto",
			config: RateLimitConfig{
				Enabled:      true,
				Conservative: true,
			},
			expectedMode: "conservative",
		},
		{
			name: "configuración por defecto",
			config: RateLimitConfig{
				Enabled:      true,
				Conservative: false,
			},
			expectedMode: "default",
		},
		{
			name: "deshabilitado",
			config: RateLimitConfig{
				Enabled:      false,
				Conservative: true,
			},
			expectedMode: "conservative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewKrakenRateLimiterFromConfig(tt.config)

			if limiter.GetMode() != tt.expectedMode {
				t.Errorf("Expected mode %s, got %s", tt.expectedMode, limiter.GetMode())
			}

			if limiter.IsEnabled() != tt.config.Enabled {
				t.Errorf("Expected enabled=%v, got %v", tt.config.Enabled, limiter.IsEnabled())
			}
		})
	}
}

func TestKrakenRateLimiter_CustomConfig(t *testing.T) {
	config := RateLimitConfig{
		Enabled:      true,
		Conservative: false,
		Capacity:     3,
		RefillRate:   1,
		RefillPeriod: 10 * time.Millisecond,
	}

	limiter := NewKrakenRateLimiterFromConfig(config)

	// Debería permitir hasta la capacidad personalizada (3)
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			allowedCount++
		}
	}

	if allowedCount != 3 {
		t.Errorf("Expected 3 allowed requests with custom config, got %d", allowedCount)
	}

	// Esperar refill
	time.Sleep(15 * time.Millisecond)

	// Debería permitir al menos 1 más después del refill
	if !limiter.Allow() {
		t.Error("Should allow request after refill period")
	}
}

// Benchmark para verificar performance del rate limiter
func BenchmarkKrakenRateLimiter_Allow(b *testing.B) {
	limiter := NewDefaultKrakenRateLimiter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

func BenchmarkKrakenRateLimiter_AllowDisabled(b *testing.B) {
	limiter := NewDefaultKrakenRateLimiter()
	limiter.Enable(false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}
