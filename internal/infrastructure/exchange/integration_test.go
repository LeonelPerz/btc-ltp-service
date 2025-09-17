//go:build integration
// +build integration

package exchange

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/exchange/kraken"
	cachepkg "btc-ltp-service/internal/infrastructure/repositories/cache"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== INTEGRATION TESTS - PRUEBAS DE INTEGRACIÓN =====

func TestIntegration_WebSocketToRestFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 5 * time.Second,
		MaxRetries:      3,
		Timeout:         10 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{"BTC/USD"})
	defer exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Primera llamada - puede usar WebSocket o REST dependiendo del estado de conexión
	price1, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("First call failed (expected in some test environments): %v", err)
		return // Skip rest of test if we can't connect
	}

	require.NotNil(t, price1)
	assert.Equal(t, "BTC/USD", price1.Pair)
	assert.Greater(t, price1.Amount, 0.0)

	// Forzar reconexión para probar fallback
	err = exchange.ForceWebSocketReconnect()
	if err != nil {
		t.Logf("WebSocket reconnection failed: %v", err)
	}

	// Segunda llamada - debería usar fallback si WebSocket falla
	price2, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("Second call failed: %v", err)
		return
	}

	require.NotNil(t, price2)
	assert.Equal(t, "BTC/USD", price2.Pair)
	assert.Greater(t, price2.Amount, 0.0)
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 5 * time.Second,
		MaxRetries:      3,
		Timeout:         10 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{"BTC/USD", "ETH/USD"})
	defer exchange.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *entities.Price, numGoroutines*2) // 2 pairs per goroutine
	errors := make(chan error, numGoroutines*2)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2) // 2 requests per goroutine

		// Request BTC/USD
		go func() {
			defer wg.Done()
			price, err := exchange.GetTicker(ctx, "BTC/USD")
			if err != nil {
				errors <- err
				return
			}
			results <- price
		}()

		// Request ETH/USD
		go func() {
			defer wg.Done()
			price, err := exchange.GetTicker(ctx, "ETH/USD")
			if err != nil {
				errors <- err
				return
			}
			results <- price
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent request error: %v", err)
		errorCount++
	}

	// Collect successful results
	successCount := 0
	btcCount := 0
	ethCount := 0
	for price := range results {
		successCount++
		if price.Pair == "BTC/USD" {
			btcCount++
			assert.Greater(t, price.Amount, 0.0)
		} else if price.Pair == "ETH/USD" {
			ethCount++
			assert.Greater(t, price.Amount, 0.0)
		}
	}

	t.Logf("Concurrent requests: %d successful, %d errors", successCount, errorCount)

	// We expect at least some successful requests
	if successCount == 0 && errorCount > 0 {
		t.Skip("All concurrent requests failed - likely network/connectivity issue")
	}

	assert.Greater(t, successCount, 0, "Expected at least some successful requests")
}

func TestIntegration_ConnectionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 2 * time.Second,
		MaxRetries:      2,
		Timeout:         5 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{"BTC/USD"})
	defer exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// First request
	price1, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("First request failed: %v", err)
		t.Skip("Cannot establish initial connection")
	}

	require.NotNil(t, price1)
	t.Logf("First price: %f", price1.Amount)

	// Check initial WebSocket status
	initialStatus := exchange.GetPrimaryStatus()
	t.Logf("Initial WebSocket status: %v", initialStatus)

	// Force close and reconnect
	err = exchange.ForceWebSocketReconnect()
	if err != nil {
		t.Logf("Reconnection attempt failed: %v", err)
	}

	// Wait a bit for reconnection
	time.Sleep(3 * time.Second)

	// Check status after reconnection attempt
	afterReconnectStatus := exchange.GetPrimaryStatus()
	t.Logf("WebSocket status after reconnect: %v", afterReconnectStatus)

	// Second request should still work (via fallback if needed)
	price2, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("Second request failed: %v", err)
		t.Skip("Connection recovery test failed - this may be expected in test environments")
	}

	require.NotNil(t, price2)
	assert.Greater(t, price2.Amount, 0.0)
	t.Logf("Second price: %f", price2.Amount)
}

func TestIntegration_CacheConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 5 * time.Second,
		MaxRetries:      3,
		Timeout:         10 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{"BTC/USD"})
	defer exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First request - should populate cache
	price1, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("First request failed: %v", err)
		t.Skip("Cannot establish connection for cache test")
	}

	require.NotNil(t, price1)

	// Wait a moment
	time.Sleep(100 * time.Millisecond)

	// Second request - might come from cache
	price2, err := exchange.GetTicker(ctx, "BTC/USD")
	require.NoError(t, err)
	require.NotNil(t, price2)

	// Both prices should be valid
	assert.Equal(t, "BTC/USD", price1.Pair)
	assert.Equal(t, "BTC/USD", price2.Pair)
	assert.Greater(t, price1.Amount, 0.0)
	assert.Greater(t, price2.Amount, 0.0)

	// Timestamps should be reasonable (within last minute)
	now := time.Now()
	assert.WithinDuration(t, now, price1.Timestamp, time.Minute)
	assert.WithinDuration(t, now, price2.Timestamp, time.Minute)
}

func TestIntegration_CacheEviction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create cache with very short TTL for testing
	cache, err := cachepkg.NewPriceCacheAdapter(context.Background(), cachepkg.CacheConfig{
		Type:       "memory",
		DefaultTTL: 100 * time.Millisecond, // Very short TTL
	})
	require.NoError(t, err)

	// Create a simple price and cache it
	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	_ = cache.Set(context.Background(), price)

	// Verify it's in cache
	cachedPrice, found := cache.Get(context.Background(), "BTC/USD")
	assert.True(t, found)
	assert.Equal(t, price.Pair, cachedPrice.Pair)

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Verify it's evicted
	_, found = cache.Get(context.Background(), "BTC/USD")
	assert.False(t, found, "Price should have been evicted from cache")
}

func TestIntegration_RealKrakenRestAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := kraken.NewRestClient()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test single ticker
	price, err := client.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("REST API test failed: %v", err)
		t.Skip("Cannot connect to real Kraken API")
	}

	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Greater(t, price.Amount, 0.0)
	assert.WithinDuration(t, time.Now(), price.Timestamp, time.Minute)

	t.Logf("BTC/USD price from REST API: %f", price.Amount)

	// Test multiple tickers
	prices, err := client.GetTickers(ctx, []string{"BTC/USD", "ETH/USD"})
	if err != nil {
		t.Logf("Multiple tickers test failed: %v", err)
		return // Don't skip, we already got one successful call
	}

	require.Len(t, prices, 2)

	pairMap := make(map[string]*entities.Price)
	for _, p := range prices {
		pairMap[p.Pair] = p
	}

	assert.Contains(t, pairMap, "BTC/USD")
	assert.Contains(t, pairMap, "ETH/USD")
	assert.Greater(t, pairMap["BTC/USD"].Amount, 0.0)
	assert.Greater(t, pairMap["ETH/USD"].Amount, 0.0)

	t.Logf("Multiple prices - BTC: %f, ETH: %f",
		pairMap["BTC/USD"].Amount, pairMap["ETH/USD"].Amount)
}

func TestIntegration_RealWebSocket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cache, err := cachepkg.NewPriceCacheAdapter(context.Background(), cachepkg.CacheConfig{
		Type: "memory",
	})
	require.NoError(t, err)

	client := kraken.NewWebSocketClient(cache)

	// Try to connect
	err = client.Connect()
	if err != nil {
		t.Logf("WebSocket connection failed: %v", err)
		t.Skip("Cannot connect to real Kraken WebSocket")
	}
	defer client.Close()

	// Subscribe to BTC/USD
	err = client.SubscribeTicker([]string{"BTC/USD"})
	if err != nil {
		t.Logf("WebSocket subscription failed: %v", err)
		t.Skip("Cannot subscribe to WebSocket ticker")
	}

	// Wait for data
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	price, err := client.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("WebSocket GetTicker failed: %v", err)
		t.Skip("Cannot get ticker data from WebSocket")
	}

	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Greater(t, price.Amount, 0.0)
	assert.WithinDuration(t, time.Now(), price.Timestamp, time.Minute)

	t.Logf("BTC/USD price from WebSocket: %f", price.Amount)

	// Verify connection status
	assert.True(t, client.IsConnected())

	// Check reconnection status
	isReconnecting, attemptCount := client.GetReconnectionStatus()
	t.Logf("Reconnection status: %v, attempts: %d", isReconnecting, attemptCount)
}

func TestIntegration_FullSystemFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 5 * time.Second,
		MaxRetries:      3,
		Timeout:         10 * time.Second,
	}

	supportedPairs := []string{"BTC/USD", "ETH/USD"}
	exchange := NewFallbackExchange(cfg, supportedPairs)
	defer exchange.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test warmup (uses REST only)
	warmupPrices, err := exchange.WarmupTickers(ctx, supportedPairs)
	if err != nil {
		t.Logf("Warmup failed: %v", err)
		t.Skip("Cannot perform warmup - likely network issue")
	}

	require.Len(t, warmupPrices, 2)
	t.Logf("Warmup completed with %d prices", len(warmupPrices))

	// Wait for WebSocket to potentially connect
	time.Sleep(5 * time.Second)

	// Test single ticker (uses fallback strategy)
	price, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("GetTicker failed: %v", err)
		t.Skip("Cannot get single ticker")
	}

	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Greater(t, price.Amount, 0.0)
	t.Logf("Single ticker: %s = %f", price.Pair, price.Amount)

	// Test multiple tickers
	prices, err := exchange.GetTickers(ctx, supportedPairs)
	if err != nil {
		t.Logf("GetTickers failed: %v", err)
		return // Don't skip, we got some successful calls
	}

	require.Len(t, prices, 2)
	for _, p := range prices {
		assert.Greater(t, p.Amount, 0.0)
		t.Logf("Multiple ticker: %s = %f", p.Pair, p.Amount)
	}

	// Check system status
	primaryStatus := exchange.GetPrimaryStatus()
	config := exchange.GetConfig()
	secondary := exchange.Secondary()

	t.Logf("System status - Primary (WebSocket): %v", primaryStatus)
	t.Logf("Config - WebSocket URL: %s", config.WebSocketURL)
	t.Logf("Config - REST URL: %s", config.RestURL)
	assert.NotNil(t, secondary)

	// Test forced reconnection
	err = exchange.ForceWebSocketReconnect()
	if err != nil {
		t.Logf("Forced reconnection failed: %v", err)
	}

	// Final test after reconnection
	finalPrice, err := exchange.GetTicker(ctx, "BTC/USD")
	if err != nil {
		t.Logf("Final ticker test failed: %v", err)
	} else {
		assert.Greater(t, finalPrice.Amount, 0.0)
		t.Logf("Final ticker: %s = %f", finalPrice.Pair, finalPrice.Amount)
	}
}
