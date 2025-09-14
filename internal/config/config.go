package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Cache  CacheConfig  `mapstructure:"cache"`
	Kraken KrakenConfig `mapstructure:"kraken"`
	Redis  RedisConfig  `mapstructure:"redis"`
	App    AppConfig    `mapstructure:"app"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port            string        `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// CacheConfig holds cache-related configuration
type CacheConfig struct {
	Backend         string        `mapstructure:"backend"`
	TTL             time.Duration `mapstructure:"ttl"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// KrakenConfig holds Kraken API-related configuration
type KrakenConfig struct {
	Timeout           time.Duration `mapstructure:"timeout"`
	BaseURL           string        `mapstructure:"base_url"`
	WebSocketEnabled  bool          `mapstructure:"websocket_enabled"`
	WebSocketURL      string        `mapstructure:"websocket_url"`
	WebSocketTimeout  time.Duration `mapstructure:"websocket_timeout"`
	ReconnectDelay    time.Duration `mapstructure:"reconnect_delay"`
	MaxReconnectTries int           `mapstructure:"max_reconnect_tries"`
}

// RedisConfig holds Redis-related configuration
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	SupportedPairs []string `mapstructure:"supported_pairs"`
	LogLevel       string   `mapstructure:"log_level"`
}

// Load loads configuration from environment variables and defaults
func Load() (*Config, error) {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.shutdown_timeout", "30s")

	viper.SetDefault("cache.backend", "memory")
	viper.SetDefault("cache.ttl", "1m")
	viper.SetDefault("cache.refresh_interval", "30s")

	viper.SetDefault("kraken.timeout", "10s")
	viper.SetDefault("kraken.base_url", "https://api.kraken.com/0/public")
	viper.SetDefault("kraken.websocket_enabled", true)
	viper.SetDefault("kraken.websocket_url", "wss://ws.kraken.com/")
	viper.SetDefault("kraken.websocket_timeout", "90s") // Further increased for network issues
	viper.SetDefault("kraken.reconnect_delay", "5s")
	viper.SetDefault("kraken.max_reconnect_tries", 5)

	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("app.supported_pairs", []string{"BTC/USD", "BTC/EUR", "BTC/CAD"})
	viper.SetDefault("app.log_level", "info")

	// Bind environment variables
	viper.BindEnv("server.port", "PORT")
	viper.BindEnv("cache.backend", "CACHE_BACKEND")
	viper.BindEnv("cache.ttl", "CACHE_TTL")
	viper.BindEnv("cache.refresh_interval", "CACHE_REFRESH_INTERVAL")
	viper.BindEnv("kraken.timeout", "KRAKEN_TIMEOUT")
	viper.BindEnv("kraken.websocket_enabled", "KRAKEN_WEBSOCKET_ENABLED")
	viper.BindEnv("kraken.websocket_url", "KRAKEN_WEBSOCKET_URL")
	viper.BindEnv("kraken.websocket_timeout", "KRAKEN_WEBSOCKET_TIMEOUT")
	viper.BindEnv("kraken.reconnect_delay", "KRAKEN_RECONNECT_DELAY")
	viper.BindEnv("kraken.max_reconnect_tries", "KRAKEN_MAX_RECONNECT_TRIES")
	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")
	viper.BindEnv("app.supported_pairs", "SUPPORTED_PAIRS")
	viper.BindEnv("app.log_level", "LOG_LEVEL")

	// Try to read from config file (optional)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Error reading config file: %v", err)
		}
		// Continue with environment variables and defaults
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetCacheConfig returns cache configuration compatible with cache package
func (c *Config) GetCacheConfig() CacheConfig {
	return c.Cache
}
