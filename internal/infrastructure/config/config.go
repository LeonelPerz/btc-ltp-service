package config

import (
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server" mapstructure:"server"`
	Cache       CacheConfig       `yaml:"cache" mapstructure:"cache"`
	Exchange    ExchangeConfig    `yaml:"exchange" mapstructure:"exchange"`
	RateLimit   RateLimitConfig   `yaml:"rate_limit" mapstructure:"rate_limit"`
	Auth        AuthConfig        `yaml:"auth" mapstructure:"auth"`
	Logging     LoggingConfig     `yaml:"logging" mapstructure:"logging"`
	Business    BusinessConfig    `yaml:"business" mapstructure:"business"`
	Development DevelopmentConfig `yaml:"development" mapstructure:"development"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port            int           `yaml:"port" mapstructure:"port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" mapstructure:"shutdown_timeout"`
}

// CacheConfig contains cache system configuration
type CacheConfig struct {
	Backend string        `yaml:"backend" mapstructure:"backend"`
	TTL     time.Duration `yaml:"ttl" mapstructure:"ttl"`
	Redis   RedisConfig   `yaml:"redis" mapstructure:"redis"`
}

// RedisConfig contains Redis-specific configuration
type RedisConfig struct {
	Addr     string `yaml:"addr" mapstructure:"addr"`
	Password string `yaml:"password" mapstructure:"password"`
	DB       int    `yaml:"db" mapstructure:"db"`
}

// ExchangeConfig contains cryptocurrency exchange configuration
type ExchangeConfig struct {
	Kraken KrakenConfig `yaml:"kraken" mapstructure:"kraken"`
}

// KrakenConfig contains Kraken-specific configuration
type KrakenConfig struct {
	RestURL         string        `yaml:"rest_url" mapstructure:"rest_url"`
	WebSocketURL    string        `yaml:"websocket_url" mapstructure:"websocket_url"`
	Timeout         time.Duration `yaml:"timeout" mapstructure:"timeout"`
	RequestTimeout  time.Duration `yaml:"request_timeout" mapstructure:"request_timeout"`
	FallbackTimeout time.Duration `yaml:"fallback_timeout" mapstructure:"fallback_timeout"`
	MaxRetries      int           `yaml:"max_retries" mapstructure:"max_retries"`
	PriceCacheTTL   time.Duration `yaml:"price_cache_ttl" mapstructure:"price_cache_ttl"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled    bool `yaml:"enabled" mapstructure:"enabled"`
	Capacity   int  `yaml:"capacity" mapstructure:"capacity"`
	RefillRate int  `yaml:"refill_rate" mapstructure:"refill_rate"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled     bool     `yaml:"enabled" mapstructure:"enabled,string"`
	APIKey      string   `yaml:"api_key" mapstructure:"api_key"`
	HeaderName  string   `yaml:"header_name" mapstructure:"header_name"`
	UnauthPaths []string `yaml:"unauth_paths" mapstructure:"unauth_paths"`
}

// LoggingConfig contains logging system configuration
type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"`
}

// BusinessConfig contains specific business configurations
type BusinessConfig struct {
	SupportedPairs []string `yaml:"supported_pairs" mapstructure:"supported_pairs"`
	CachePrefix    string   `yaml:"cache_prefix" mapstructure:"cache_prefix"`
}

// DevelopmentConfig contiene configuraciones para desarrollo y testing
type DevelopmentConfig struct {
	MockMode  bool `yaml:"mock_mode" mapstructure:"mock_mode"`
	DebugMode bool `yaml:"debug_mode" mapstructure:"debug_mode"`
	DevMode   bool `yaml:"dev_mode" mapstructure:"dev_mode"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			ShutdownTimeout: 30 * time.Second,
		},
		Cache: CacheConfig{
			Backend: "memory",
			TTL:     30 * time.Second,
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "",
				DB:       0,
			},
		},
		Exchange: ExchangeConfig{
			Kraken: KrakenConfig{
				RestURL:         "https://api.kraken.com/0/public",
				WebSocketURL:    "wss://ws.kraken.com",
				Timeout:         10 * time.Second,
				RequestTimeout:  3 * time.Second,
				FallbackTimeout: 15 * time.Second,
				MaxRetries:      3,
				PriceCacheTTL:   30 * time.Second,
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:    true,
			Capacity:   100,
			RefillRate: 10,
		},
		Auth: AuthConfig{
			Enabled:     false, // Disabled by default
			APIKey:      "",
			HeaderName:  "X-API-Key",
			UnauthPaths: []string{"/health", "/ready", "/metrics", "/swagger/", "/docs"},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Business: BusinessConfig{
			SupportedPairs: []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD"},
			CachePrefix:    "price:",
		},
		Development: DevelopmentConfig{
			MockMode:  false,
			DebugMode: false,
			DevMode:   false,
		},
	}
}
