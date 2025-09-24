package exchange

import (
	"btc-ltp-service/internal/infrastructure/config"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFallbackExchange_ResilienceMatrix tests the resilience matrix scenarios
func TestFallbackExchange_ResilienceMatrix(t *testing.T) {
	tests := []struct {
		name           string
		wsTimeout      time.Duration
		maxRetries     int
		simulateWSFail bool
		expectFallback bool
		expectError    bool
		description    string
	}{
		{
			name:           "Normal Operation - WebSocket Success",
			wsTimeout:      time.Second * 5,
			maxRetries:     3,
			simulateWSFail: false,
			expectFallback: false,
			expectError:    false,
			description:    "WebSocket responde correctamente, sin fallback",
		},
		{
			name:           "Timeout Scenario - Fast Timeout",
			wsTimeout:      time.Millisecond * 10, // Very short timeout
			maxRetries:     2,
			simulateWSFail: true,
			expectFallback: true,
			expectError:    false,
			description:    "WebSocket timeout rápido activa fallback a REST",
		},
		{
			name:           "Max Retries Scenario",
			wsTimeout:      time.Second * 1,
			maxRetries:     1, // Only 1 retry
			simulateWSFail: true,
			expectFallback: true,
			expectError:    false,
			description:    "Máximo de reintentos alcanzado activa fallback",
		},
		{
			name:           "Connection Error Scenario",
			wsTimeout:      time.Second * 2,
			maxRetries:     2,
			simulateWSFail: true,
			expectFallback: true,
			expectError:    false,
			description:    "Error de conexión WebSocket activa fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test configuration with specific timeouts
			krakenConfig := config.KrakenConfig{
				RestURL:         "https://api.kraken.com/0/public",
				WebSocketURL:    "wss://ws.kraken.com",
				Timeout:         time.Second * 10,
				RequestTimeout:  time.Second * 3,
				FallbackTimeout: tt.wsTimeout,
				MaxRetries:      tt.maxRetries,
				PriceCacheTTL:   time.Second * 30,
			}

			// Create fallback exchange
			exchange := NewFallbackExchange(krakenConfig, []string{"BTC/USD"})
			defer exchange.Close()

			// Wait for potential WebSocket connection
			time.Sleep(time.Millisecond * 50)

			// Test single price request
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			startTime := time.Now()
			price, err := exchange.GetTicker(ctx, "BTC/USD")
			duration := time.Since(startTime)

			if tt.expectError {
				assert.Error(t, err, "Expected error for test case: %s", tt.description)
				assert.Nil(t, price, "Price should be nil when error expected")
			} else {
				assert.NoError(t, err, "No error expected for test case: %s", tt.description)
				assert.NotNil(t, price, "Price should not be nil for successful case")

				if price != nil {
					assert.Equal(t, "BTC/USD", price.Pair)
					assert.Greater(t, price.Amount, 0.0)
				}
			}

			// Verify timing expectations
			if tt.expectFallback {
				// Fallback should be relatively quick but include retry time
				expectedMinTime := tt.wsTimeout * time.Duration(tt.maxRetries)
				assert.GreaterOrEqual(t, duration, expectedMinTime,
					"Fallback should take at least the sum of timeouts and retries")
			}

			t.Logf("Test '%s' completed in %v - %s", tt.name, duration, tt.description)
		})
	}
}

// TestFallbackExchange_CircuitBreakerThresholds tests specific circuit breaker threshold scenarios
func TestFallbackExchange_CircuitBreakerThresholds(t *testing.T) {
	t.Run("Timeout Threshold Test", func(t *testing.T) {
		// Very aggressive timeout to force fallback
		krakenConfig := config.KrakenConfig{
			RestURL:         "https://api.kraken.com/0/public",
			WebSocketURL:    "wss://invalid-ws-url-for-test.com", // Invalid URL
			Timeout:         time.Second * 10,
			RequestTimeout:  time.Second * 3,
			FallbackTimeout: time.Millisecond * 50, // Very short timeout
			MaxRetries:      1,
			PriceCacheTTL:   time.Second * 30,
		}

		exchange := NewFallbackExchange(krakenConfig, []string{})
		defer exchange.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// This should trigger fallback due to timeout
		price, err := exchange.GetTicker(ctx, "BTC/USD")

		// Should succeed via REST fallback
		require.NoError(t, err)
		require.NotNil(t, price)
		assert.Equal(t, "BTC/USD", price.Pair)
	})

	t.Run("MaxRetries Threshold Test", func(t *testing.T) {
		krakenConfig := config.KrakenConfig{
			RestURL:         "https://api.kraken.com/0/public",
			WebSocketURL:    "wss://invalid-ws-url.com",
			Timeout:         time.Second * 10,
			RequestTimeout:  time.Second * 3,
			FallbackTimeout: time.Millisecond * 100,
			MaxRetries:      2, // Test with 2 retries
			PriceCacheTTL:   time.Second * 30,
		}

		exchange := NewFallbackExchange(krakenConfig, []string{})
		defer exchange.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		startTime := time.Now()
		price, err := exchange.GetTicker(ctx, "BTC/USD")
		duration := time.Since(startTime)

		require.NoError(t, err)
		require.NotNil(t, price)

		// Should take at least 2 retries * timeout duration
		expectedMinDuration := time.Duration(krakenConfig.MaxRetries) * krakenConfig.FallbackTimeout
		assert.GreaterOrEqual(t, duration, expectedMinDuration,
			"Duration should include retry attempts: expected >= %v, got %v", expectedMinDuration, duration)
	})
}

// TestFallbackExchange_MultipleRequestsResilience tests resilience under load
func TestFallbackExchange_MultipleRequestsResilience(t *testing.T) {
	krakenConfig := config.KrakenConfig{
		RestURL:         "https://api.kraken.com/0/public",
		WebSocketURL:    "wss://invalid-for-test.com", // Force fallback
		Timeout:         time.Second * 10,
		RequestTimeout:  time.Second * 3,
		FallbackTimeout: time.Millisecond * 100,
		MaxRetries:      1,
		PriceCacheTTL:   time.Second * 30,
	}

	exchange := NewFallbackExchange(krakenConfig, []string{})
	defer exchange.Close()

	// Test multiple pairs at once
	pairs := []string{"BTC/USD", "ETH/USD", "LTC/USD"}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	startTime := time.Now()
	prices, err := exchange.GetTickers(ctx, pairs)
	duration := time.Since(startTime)

	require.NoError(t, err)
	require.NotNil(t, prices)
	assert.Len(t, prices, len(pairs))

	// Verify all prices are valid
	for i, price := range prices {
		assert.NotNil(t, price, "Price %d should not be nil", i)
		if price != nil {
			assert.Contains(t, pairs, price.Pair, "Price pair should be one of the requested pairs")
			assert.Greater(t, price.Amount, 0.0, "Price amount should be positive")
		}
	}

	t.Logf("Multiple pairs fallback completed in %v for %d pairs", duration, len(pairs))
}

// TestFallbackExchange_DetermineFallbackReason tests the fallback reason determination logic
func TestFallbackExchange_DetermineFallbackReason(t *testing.T) {
	exchange := &FallbackExchange{}

	tests := []struct {
		name           string
		err            error
		expectedReason string
	}{
		{
			name:           "Timeout Error",
			err:            errors.New("WebSocket timeout after 5s"),
			expectedReason: "timeout",
		},
		{
			name:           "Connection Error",
			err:            errors.New("connection refused"),
			expectedReason: "connection_error",
		},
		{
			name:           "Max Retries Error",
			err:            errors.New("max retries exceeded"),
			expectedReason: "max_retries",
		},
		{
			name:           "Panic Recovery",
			err:            errors.New("WebSocket panic recovered"),
			expectedReason: "panic",
		},
		{
			name:           "Connection Closed",
			err:            errors.New("connection closed"),
			expectedReason: "connection_closed",
		},
		{
			name:           "Unknown Error",
			err:            errors.New("some random error"),
			expectedReason: "unknown_error",
		},
		{
			name:           "Nil Error",
			err:            nil,
			expectedReason: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := exchange.determineFallbackReason(tt.err)
			assert.Equal(t, tt.expectedReason, reason)
		})
	}
}

// TestFallbackExchange_WebSocketStatus tests WebSocket connection status reporting
func TestFallbackExchange_WebSocketStatus(t *testing.T) {
	t.Run("Valid WebSocket URL", func(t *testing.T) {
		krakenConfig := config.KrakenConfig{
			RestURL:         "https://api.kraken.com/0/public",
			WebSocketURL:    "wss://ws.kraken.com", // Valid URL
			Timeout:         time.Second * 10,
			RequestTimeout:  time.Second * 3,
			FallbackTimeout: time.Second * 5,
			MaxRetries:      3,
			PriceCacheTTL:   time.Second * 30,
		}

		exchange := NewFallbackExchange(krakenConfig, []string{})
		defer exchange.Close()

		// Give some time for connection attempt
		time.Sleep(time.Millisecond * 100)

		// Note: The actual connection may still fail due to network/auth issues
		// This test primarily verifies the method works without panicking
		status := exchange.GetPrimaryStatus()
		assert.IsType(t, true, status) // Just verify it returns a boolean
	})

	t.Run("Invalid WebSocket URL", func(t *testing.T) {
		krakenConfig := config.KrakenConfig{
			RestURL:         "https://api.kraken.com/0/public",
			WebSocketURL:    "wss://invalid-url-test.com",
			Timeout:         time.Second * 10,
			RequestTimeout:  time.Second * 3,
			FallbackTimeout: time.Millisecond * 50,
			MaxRetries:      1,
			PriceCacheTTL:   time.Second * 30,
		}

		exchange := NewFallbackExchange(krakenConfig, []string{})
		defer exchange.Close()

		// Give some time for connection attempt to fail
		time.Sleep(time.Millisecond * 100)

		status := exchange.GetPrimaryStatus()
		// With invalid URL, should be disconnected
		assert.False(t, status)
	})
}

// TestFallbackExchange_ConfigurationValidation tests different configuration scenarios
func TestFallbackExchange_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      config.KrakenConfig
		expectPanic bool
		description string
	}{
		{
			name: "Standard Production Config",
			config: config.KrakenConfig{
				RestURL:         "https://api.kraken.com/0/public",
				WebSocketURL:    "wss://ws.kraken.com",
				Timeout:         time.Second * 10,
				RequestTimeout:  time.Second * 3,
				FallbackTimeout: time.Second * 15,
				MaxRetries:      3,
				PriceCacheTTL:   time.Second * 30,
			},
			expectPanic: false,
			description: "Standard production configuration should work",
		},
		{
			name: "Fast Development Config",
			config: config.KrakenConfig{
				RestURL:         "https://api.kraken.com/0/public",
				WebSocketURL:    "wss://ws.kraken.com",
				Timeout:         time.Second * 5,
				RequestTimeout:  time.Second * 2,
				FallbackTimeout: time.Second * 5,
				MaxRetries:      2,
				PriceCacheTTL:   time.Second * 10,
			},
			expectPanic: false,
			description: "Fast development configuration should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					exchange := NewFallbackExchange(tt.config, []string{})
					exchange.Close()
				}, tt.description)
			} else {
				assert.NotPanics(t, func() {
					exchange := NewFallbackExchange(tt.config, []string{})
					defer exchange.Close()

					// Quick test to ensure basic functionality works
					ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
					defer cancel()

					// This might use fallback, which is fine
					_, err := exchange.GetTicker(ctx, "BTC/USD")

					// We allow errors here since we're testing configuration validation,
					// not the actual API calls
					t.Logf("Configuration test '%s': %v - %s", tt.name, err, tt.description)
				}, tt.description)
			}
		})
	}
}

// BenchmarkFallbackExchange_GetTicker benchmarks the fallback mechanism performance
func BenchmarkFallbackExchange_GetTicker(b *testing.B) {
	krakenConfig := config.KrakenConfig{
		RestURL:         "https://api.kraken.com/0/public",
		WebSocketURL:    "wss://invalid-for-benchmark.com", // Force REST usage
		Timeout:         time.Second * 10,
		RequestTimeout:  time.Second * 3,
		FallbackTimeout: time.Millisecond * 50, // Quick timeout for benchmark
		MaxRetries:      1,
		PriceCacheTTL:   time.Second * 30,
	}

	exchange := NewFallbackExchange(krakenConfig, []string{})
	defer exchange.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := exchange.GetTicker(ctx, "BTC/USD")
			if err != nil {
				// In benchmark, we expect some errors due to rate limiting
				// Don't fail the benchmark, just continue
				continue
			}
		}
	})
}

// TestFallbackExchange_ErrorScenarios tests various error scenarios
func TestFallbackExchange_ErrorScenarios(t *testing.T) {
	t.Run("Both WebSocket and REST Fail", func(t *testing.T) {
		// This test requires mocking or using a test server
		// For now, we'll test the error handling logic

		krakenConfig := config.KrakenConfig{
			RestURL:         "https://invalid-rest-api.com/0/public", // Invalid REST URL
			WebSocketURL:    "wss://invalid-ws.com",                  // Invalid WS URL
			Timeout:         time.Second * 1,
			RequestTimeout:  time.Millisecond * 500,
			FallbackTimeout: time.Millisecond * 100,
			MaxRetries:      1,
			PriceCacheTTL:   time.Second * 30,
		}

		exchange := NewFallbackExchange(krakenConfig, []string{})
		defer exchange.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, err := exchange.GetTicker(ctx, "BTC/USD")

		// Should get an error when both fail
		require.Error(t, err)
		assert.True(t,
			strings.Contains(err.Error(), "both WebSocket and REST failed"),
			"Error should indicate both systems failed: %v", err)
	})
}
