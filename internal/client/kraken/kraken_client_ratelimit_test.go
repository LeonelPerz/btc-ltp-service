package kraken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"btc-ltp-service/internal/model"
	"btc-ltp-service/internal/ratelimit"
)

func TestClient_RateLimit_Integration(t *testing.T) {
	// Inicializar pares soportados para los tests
	if err := model.InitializeSupportedPairs([]string{"BTC/USD"}); err != nil {
		t.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	// Crear servidor mock que cuenta las peticiones
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Respuesta mock válida de Kraken
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":[],"result":{"XXBTZUSD":{"c":["50000.0","0"]}}}`))
	}))
	defer server.Close()

	// Crear cliente con rate limiting muy restrictivo para pruebas
	config := ratelimit.RateLimitConfig{
		Enabled:      true,
		Conservative: false,
		Capacity:     2,                      // Solo 2 tokens
		RefillRate:   1,                      // 1 token por período
		RefillPeriod: 100 * time.Millisecond, // Cada 100ms
	}
	rateLimiter := ratelimit.NewKrakenRateLimiterFromConfig(config)

	client := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:     server.URL,
		timeout:     5 * time.Second,
		rateLimiter: rateLimiter,
	}

	pairs := []string{"BTC/USD"}

	// Hacer múltiples peticiones rápidamente
	start := time.Now()
	var wg sync.WaitGroup
	successCount := 0
	var successMu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := client.GetTickerDataWithContext(ctx, pairs)
			if err == nil {
				successMu.Lock()
				successCount++
				successMu.Unlock()
			} else {
				t.Logf("Request %d failed: %v", i, err)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	mu.Lock()
	finalRequestCount := requestCount
	mu.Unlock()

	successMu.Lock()
	finalSuccessCount := successCount
	successMu.Unlock()

	t.Logf("Completed %d successful requests out of 5 attempts", finalSuccessCount)
	t.Logf("Server received %d total requests", finalRequestCount)
	t.Logf("Total elapsed time: %v", elapsed)

	// Con rate limiting, no deberían pasar todas las peticiones inmediatamente
	// Debería tomar más tiempo debido al rate limiting
	if elapsed < 50*time.Millisecond {
		t.Errorf("Rate limiting should have caused some delay, but completed in %v", elapsed)
	}

	// Todas las peticiones exitosas deberían haber llegado al servidor
	if finalSuccessCount != finalRequestCount {
		t.Errorf("Mismatch between successful requests (%d) and server requests (%d)",
			finalSuccessCount, finalRequestCount)
	}
}

func TestClient_RateLimit_Disabled(t *testing.T) {
	// Inicializar pares soportados para los tests
	if err := model.InitializeSupportedPairs([]string{"BTC/USD"}); err != nil {
		t.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	// Crear servidor mock
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":[],"result":{"XXBTZUSD":{"c":["50000.0","0"]}}}`))
	}))
	defer server.Close()

	// Crear cliente sin rate limiting
	client := NewClientWithoutRateLimit(5 * time.Second)
	client.baseURL = server.URL

	pairs := []string{"BTC/USD"}

	// Hacer múltiples peticiones rápidamente
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.GetTickerData(pairs)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Without rate limiting: %d requests completed in %v", requestCount, elapsed)

	// Sin rate limiting, debería completarse muy rápido
	if elapsed > 100*time.Millisecond {
		t.Errorf("Without rate limiting should be fast, took %v", elapsed)
	}

	if requestCount != 5 {
		t.Errorf("Expected 5 requests, got %d", requestCount)
	}
}

func TestClient_RateLimit_Stats(t *testing.T) {
	client := NewClient()

	stats := client.GetRateLimitStats()

	// Verificar que las estadísticas tienen los campos esperados
	expectedFields := []string{"enabled", "mode", "current_tokens", "capacity"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Rate limit stats should include field: %s", field)
		}
	}

	if !client.IsRateLimitEnabled() {
		t.Error("Rate limiting should be enabled by default")
	}

	mode := client.GetRateLimitMode()
	if mode != "conservative" {
		t.Errorf("Expected conservative mode by default, got %s", mode)
	}
}

func TestClient_RateLimit_EnableDisable(t *testing.T) {
	client := NewClient()

	// Inicialmente habilitado
	if !client.IsRateLimitEnabled() {
		t.Error("Rate limiting should be enabled initially")
	}

	// Deshabilitar
	client.EnableRateLimit(false)
	if client.IsRateLimitEnabled() {
		t.Error("Rate limiting should be disabled after EnableRateLimit(false)")
	}

	// Habilitar nuevamente
	client.EnableRateLimit(true)
	if !client.IsRateLimitEnabled() {
		t.Error("Rate limiting should be enabled after EnableRateLimit(true)")
	}
}

func TestClient_RateLimit_Sequential(t *testing.T) {
	// Inicializar pares soportados para los tests
	if err := model.InitializeSupportedPairs([]string{"BTC/USD"}); err != nil {
		t.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	// Test para verificar que las peticiones secuenciales respetan el rate limit
	var timestamps []time.Time
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":[],"result":{"XXBTZUSD":{"c":["50000.0","0"]}}}`))
	}))
	defer server.Close()

	// Cliente con rate limiting conservador
	config := ratelimit.RateLimitConfig{
		Enabled:      true,
		Conservative: false,
		Capacity:     1,                      // Solo 1 token
		RefillRate:   1,                      // 1 token por período
		RefillPeriod: 200 * time.Millisecond, // Cada 200ms
	}
	rateLimiter := ratelimit.NewKrakenRateLimiterFromConfig(config)

	client := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:     server.URL,
		timeout:     5 * time.Second,
		rateLimiter: rateLimiter,
	}

	pairs := []string{"BTC/USD"}

	// Hacer 3 peticiones secuenciales
	for i := 0; i < 3; i++ {
		_, err := client.GetTickerData(pairs)
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	mu.Lock()
	requestTimestamps := make([]time.Time, len(timestamps))
	copy(requestTimestamps, timestamps)
	mu.Unlock()

	if len(requestTimestamps) < 3 {
		t.Fatalf("Expected at least 3 requests, got %d", len(requestTimestamps))
	}

	// Verificar que hay espaciado entre requests debido al rate limiting
	for i := 1; i < len(requestTimestamps); i++ {
		gap := requestTimestamps[i].Sub(requestTimestamps[i-1])
		t.Logf("Gap between request %d and %d: %v", i-1, i, gap)

		// Debería haber al menos algo de delay debido al rate limiting
		if gap < 50*time.Millisecond {
			t.Errorf("Expected rate limiting delay between requests, got gap: %v", gap)
		}
	}
}

// Benchmark para medir el overhead del rate limiting
func BenchmarkClient_WithRateLimit(b *testing.B) {
	// Inicializar pares soportados para los tests
	if err := model.InitializeSupportedPairs([]string{"BTC/USD"}); err != nil {
		b.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":[],"result":{"XXBTZUSD":{"c":["50000.0","0"]}}}`))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	pairs := []string{"BTC/USD"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetTickerData(pairs)
	}
}

func BenchmarkClient_WithoutRateLimit(b *testing.B) {
	// Inicializar pares soportados para los tests
	if err := model.InitializeSupportedPairs([]string{"BTC/USD"}); err != nil {
		b.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error":[],"result":{"XXBTZUSD":{"c":["50000.0","0"]}}}`))
	}))
	defer server.Close()

	client := NewClientWithoutRateLimit(5 * time.Second)
	client.baseURL = server.URL
	pairs := []string{"BTC/USD"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetTickerData(pairs)
	}
}
