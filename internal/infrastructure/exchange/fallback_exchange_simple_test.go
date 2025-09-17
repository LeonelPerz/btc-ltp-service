package exchange

import (
	"btc-ltp-service/internal/infrastructure/config"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== CASOS DE ÉXITO - FUNCIONAMIENTO NORMAL (SIMPLIFICADO) =====

func TestNewFallbackExchange_Success_Simple(t *testing.T) {
	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 2 * time.Second,
		MaxRetries:      3,
		Timeout:         10 * time.Second,
	}
	supportedPairs := []string{"BTC/USD", "ETH/USD"}

	exchange := NewFallbackExchange(cfg, supportedPairs)

	assert.NotNil(t, exchange)
	assert.NotNil(t, exchange.primary)
	assert.NotNil(t, exchange.secondary)
	assert.Equal(t, cfg, exchange.config)

	// Test configuration access
	retrievedConfig := exchange.GetConfig()
	assert.Equal(t, cfg, retrievedConfig)

	// Test secondary access
	secondary := exchange.Secondary()
	assert.NotNil(t, secondary)

	// Test primary status (should be false initially)
	status := exchange.GetPrimaryStatus()
	assert.False(t, status) // WebSocket not connected yet

	// Clean up
	err := exchange.Close()
	assert.NoError(t, err)
}

func TestFallbackExchange_GetTickers_EmptyPairs_Simple(t *testing.T) {
	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 1 * time.Second,
		MaxRetries:      1,
		Timeout:         5 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{})
	defer func() {
		_ = exchange.Close()
	}()

	// Test empty pairs
	prices, err := exchange.GetTickers(context.TODO(), []string{})
	require.NoError(t, err)
	assert.Empty(t, prices)
}

func TestFallbackExchange_Close_Success_Simple(t *testing.T) {
	cfg := config.KrakenConfig{
		WebSocketURL:    "wss://ws.kraken.com",
		RestURL:         "https://api.kraken.com/0/public",
		FallbackTimeout: 1 * time.Second,
		MaxRetries:      1,
		Timeout:         5 * time.Second,
	}

	exchange := NewFallbackExchange(cfg, []string{})

	err := exchange.Close()
	assert.NoError(t, err)

	// Should be able to close multiple times without error
	err = exchange.Close()
	assert.NoError(t, err)
}

// ===== EDGE CASES - CASOS LÍMITE (SIMPLIFICADO) =====

func TestFallbackExchange_NilPrimary_Simple(t *testing.T) {
	exchange := &FallbackExchange{
		primary: nil,
	}

	status := exchange.GetPrimaryStatus()
	assert.False(t, status)
}

// ===== CONFIGURATION TESTS (SIMPLIFICADO) =====

func TestFallbackExchange_ConfigurationValues(t *testing.T) {
	testCases := []struct {
		name string
		cfg  config.KrakenConfig
	}{
		{
			name: "Default values",
			cfg: config.KrakenConfig{
				WebSocketURL:    "wss://ws.kraken.com",
				RestURL:         "https://api.kraken.com/0/public",
				FallbackTimeout: 5 * time.Second,
				MaxRetries:      3,
				Timeout:         10 * time.Second,
			},
		},
		{
			name: "Custom values",
			cfg: config.KrakenConfig{
				WebSocketURL:    "wss://custom.kraken.com",
				RestURL:         "https://custom-api.kraken.com/0/public",
				FallbackTimeout: 1 * time.Second,
				MaxRetries:      1,
				Timeout:         3 * time.Second,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exchange := NewFallbackExchange(tc.cfg, []string{})
			defer func() {
				_ = exchange.Close()
			}()

			retrievedConfig := exchange.GetConfig()
			assert.Equal(t, tc.cfg.WebSocketURL, retrievedConfig.WebSocketURL)
			assert.Equal(t, tc.cfg.RestURL, retrievedConfig.RestURL)
			assert.Equal(t, tc.cfg.FallbackTimeout, retrievedConfig.FallbackTimeout)
			assert.Equal(t, tc.cfg.MaxRetries, retrievedConfig.MaxRetries)
			assert.Equal(t, tc.cfg.Timeout, retrievedConfig.Timeout)
		})
	}
}
