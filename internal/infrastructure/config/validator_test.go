package config

import (
	"strings"
	"testing"
	"time"
)

// TestValidateTTL_FailFast tests TTL validation with specific edge cases
func TestValidateTTL_FailFast(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		ttl           time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:        "Válido - TTL normal",
			ttl:         30 * time.Second,
			expectError: false,
		},
		{
			name:          "Inválido - TTL negativo",
			ttl:           -1 * time.Second,
			expectError:   true,
			errorContains: "TTL must be positive",
		},
		{
			name:          "Inválido - TTL zero",
			ttl:           0,
			expectError:   true,
			errorContains: "TTL must be positive",
		},
		{
			name:          "Inválido - TTL muy corto (50ms)",
			ttl:           50 * time.Millisecond,
			expectError:   true,
			errorContains: "TTL too short",
		},
		{
			name:          "Inválido - TTL extremadamente corto (1ms)",
			ttl:           1 * time.Millisecond,
			expectError:   true,
			errorContains: "TTL too short",
		},
		{
			name:          "Inválido - TTL muy largo (25h)",
			ttl:           25 * time.Hour,
			expectError:   true,
			errorContains: "TTL too long",
		},
		{
			name:          "Subóptimo - TTL potencialmente ineficiente (500ms)",
			ttl:           500 * time.Millisecond,
			expectError:   true,
			errorContains: "potentially inefficient",
		},
		{
			name:          "Subóptimo - TTL potencialmente stale (2h)",
			ttl:           2 * time.Hour,
			expectError:   true,
			errorContains: "potentially stale",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTTL(tt.ttl)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for TTL %v, but got none", tt.ttl)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for TTL %v, got: %v", tt.ttl, err)
				}
			}
		})
	}
}

// TestValidateTradingPairs_FailFast tests trading pairs validation
func TestValidateTradingPairs_FailFast(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		pairs         []string
		expectError   bool
		errorContains string
	}{
		{
			name:        "Válido - Pares conocidos",
			pairs:       []string{"BTC/USD", "ETH/USD", "LTC/USD"},
			expectError: false,
		},
		{
			name:        "Válido - Pares conocidos case insensitive",
			pairs:       []string{"btc/usd", "ETH/eur", "XRP/USD"},
			expectError: false,
		},
		{
			name:          "Inválido - Lista vacía",
			pairs:         []string{},
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "Inválido - Formato sin slash",
			pairs:         []string{"BTCUSD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
		},
		{
			name:          "Inválido - Formato con slash doble",
			pairs:         []string{"BTC//USD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
		},
		{
			name:          "Inválido - Base vacía",
			pairs:         []string{"/USD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
		},
		{
			name:          "Inválido - Quote vacía",
			pairs:         []string{"BTC/", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
		},
		{
			name:          "Inválido - Par desconocido",
			pairs:         []string{"BTC/USD", "DOGE/MOON"},
			expectError:   true,
			errorContains: "unknown trading pairs",
		},
		{
			name:          "Inválido - Múltiples pares desconocidos",
			pairs:         []string{"FAKE/COIN", "INVALID/PAIR", "BTC/USD"},
			expectError:   true,
			errorContains: "unknown trading pairs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTradingPairs(tt.pairs)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for pairs %v, but got none", tt.pairs)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for pairs %v, got: %v", tt.pairs, err)
				}
			}
		})
	}
}

// TestValidateCache_IntegrationFailFast tests cache validation integration
func TestValidateCache_IntegrationFailFast(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		config        CacheConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "Válido - Configuración normal",
			config: CacheConfig{
				Backend: "memory",
				TTL:     30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "Inválido - Backend desconocido",
			config: CacheConfig{
				Backend: "unknown",
				TTL:     30 * time.Second,
			},
			expectError:   true,
			errorContains: "invalid cache backend",
		},
		{
			name: "Inválido - TTL muy corto",
			config: CacheConfig{
				Backend: "memory",
				TTL:     50 * time.Millisecond,
			},
			expectError:   true,
			errorContains: "cache TTL validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateCache(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for config %+v, but got none", tt.config)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for config %+v, got: %v", tt.config, err)
				}
			}
		})
	}
}

// TestKnownKrakenPairs verifica que los pares conocidos están correctos
func TestKnownKrakenPairs(t *testing.T) {
	validator := NewValidator()
	knownPairs := validator.getKnownKrakenPairs()

	// Verificar que tenemos pares básicos
	expectedPairs := []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD", "BTC/EUR", "ETH/EUR"}

	for _, pair := range expectedPairs {
		if !knownPairs[pair] {
			t.Errorf("Expected pair %s to be in known pairs", pair)
		}
	}

	// Verificar que al menos tenemos un número razonable de pares
	if len(knownPairs) < 10 {
		t.Errorf("Expected at least 10 known pairs, got %d", len(knownPairs))
	}
}

// TestValidateConfigIntegrity_ParseErrors tests detection of parsing errors
func TestValidateConfigIntegrity_ParseErrors(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		config        Config
		expectError   bool
		errorContains string
	}{
		{
			name: "Válido - Configuración normal",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Cache:  CacheConfig{TTL: 30 * time.Second},
				Exchange: ExchangeConfig{
					Kraken: KrakenConfig{Timeout: 10 * time.Second},
				},
				Business: BusinessConfig{CachePrefix: "price:"},
			},
			expectError: false,
		},
		{
			name: "Inválido - TTL parseado como 0 (string inválido)",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Cache:  CacheConfig{TTL: 0}, // TTL 0 indica parsing error
				Exchange: ExchangeConfig{
					Kraken: KrakenConfig{Timeout: 10 * time.Second},
				},
				Business: BusinessConfig{CachePrefix: "price:"},
			},
			expectError:   true,
			errorContains: "cache TTL parsed as 0",
		},
		{
			name: "Inválido - Puerto parseado como 0",
			config: Config{
				Server: ServerConfig{Port: 0}, // Puerto 0 indica parsing error
				Cache:  CacheConfig{TTL: 30 * time.Second},
				Exchange: ExchangeConfig{
					Kraken: KrakenConfig{Timeout: 10 * time.Second},
				},
				Business: BusinessConfig{CachePrefix: "price:"},
			},
			expectError:   true,
			errorContains: "server port parsed as 0",
		},
		{
			name: "Inválido - Timeout parseado como 0",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Cache:  CacheConfig{TTL: 30 * time.Second},
				Exchange: ExchangeConfig{
					Kraken: KrakenConfig{Timeout: 0}, // Timeout 0 indica parsing error
				},
				Business: BusinessConfig{CachePrefix: "price:"},
			},
			expectError:   true,
			errorContains: "kraken timeout parsed as 0",
		},
		{
			name: "Inválido - Cache prefix vacío",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Cache:  CacheConfig{TTL: 30 * time.Second},
				Exchange: ExchangeConfig{
					Kraken: KrakenConfig{Timeout: 10 * time.Second},
				},
				Business: BusinessConfig{CachePrefix: ""}, // Prefix vacío
			},
			expectError:   true,
			errorContains: "cache prefix is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateConfigIntegrity(&tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for config %+v, but got none", tt.config)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for config %+v, got: %v", tt.config, err)
				}
			}
		})
	}
}
