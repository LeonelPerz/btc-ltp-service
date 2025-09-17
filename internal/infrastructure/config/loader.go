package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Loader handles configuration loading using Viper
type Loader struct {
	v *viper.Viper
}

// NewLoader creates a new configuration loader instance
func NewLoader() *Loader {
	return &Loader{
		v: viper.New(),
	}
}

// Load loads configuration from files and environment variables
func (l *Loader) Load() (*Config, error) {
	// 1. Configure Viper
	if err := l.setupViper(); err != nil {
		return nil, fmt.Errorf("failed to setup viper: %w", err)
	}

	// 2. Read configuration
	if err := l.v.ReadInConfig(); err != nil {
		// If config.yaml doesn't exist, use only env vars and defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// 3. Unmarshall a struct
	config := GetDefaultConfig()
	if err := l.v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 4. Override with specific env vars (for compatibility)
	l.overrideWithEnvVars(config)

	return config, nil
}

// setupViper configures Viper to read files and env vars
func (l *Loader) setupViper() error {
	// Configure to read YAML files
	l.v.SetConfigName("config")
	l.v.SetConfigType("yaml")

	// Search for configuration files in:
	l.v.AddConfigPath("./configs")    // Configs directory in root
	l.v.AddConfigPath("../configs")   // For when running from cmd/
	l.v.AddConfigPath(".")            // Current directory
	l.v.AddConfigPath("/etc/btc-ltp") // System (production)

	// Automatic environment variables
	l.v.AutomaticEnv()
	l.v.SetEnvPrefix("BTC_LTP") // Prefix for env vars: BTC_LTP_SERVER_PORT
	l.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicit env vars mapping (for backward compatibility)
	l.bindEnvVars()

	return nil
}

// bindEnvVars maps specific environment variables to configuration keys
func (l *Loader) bindEnvVars() {
	// Existing environment variables (backward compatibility)
	envMappings := map[string]string{
		"server.port":                      "PORT",
		"cache.backend":                    "CACHE_BACKEND",
		"cache.ttl":                        "CACHE_TTL",
		"cache.redis.addr":                 "REDIS_ADDR",
		"cache.redis.password":             "REDIS_PASSWORD",
		"cache.redis.db":                   "REDIS_DB",
		"business.supported_pairs":         "SUPPORTED_PAIRS",
		"exchange.kraken.rest_url":         "KRAKEN_BASE_URL",
		"exchange.kraken.timeout":          "KRAKEN_TIMEOUT",
		"exchange.kraken.fallback_timeout": "KRAKEN_FALLBACK_TIMEOUT",
		"exchange.kraken.price_cache_ttl":  "PRICE_CACHE_TTL",
		"logging.level":                    "LOG_LEVEL",
		"logging.format":                   "LOG_FORMAT",
		"rate_limit.capacity":              "RATE_LIMIT_CAPACITY",
		"rate_limit.refill_rate":           "RATE_LIMIT_REFILL_RATE",
		"rate_limit.enabled":               "RATE_LIMIT_ENABLED",
	}

	for configKey, envVar := range envMappings {
		_ = l.v.BindEnv(configKey, envVar)
	}
}

// overrideWithEnvVars maneja casos especiales de env vars
func (l *Loader) overrideWithEnvVars(config *Config) {
	// SUPPORTED_PAIRS como string separado por comas
	if supportedPairsEnv := os.Getenv("SUPPORTED_PAIRS"); supportedPairsEnv != "" {
		pairs := strings.Split(supportedPairsEnv, ",")
		var cleanPairs []string

		for _, pair := range pairs {
			pair = strings.TrimSpace(strings.ToUpper(pair))
			if pair != "" && strings.Contains(pair, "/") {
				cleanPairs = append(cleanPairs, pair)
			}
		}

		if len(cleanPairs) > 0 {
			config.Business.SupportedPairs = cleanPairs
		}
	}

	// Development mode env vars
	if devMode := os.Getenv("DEV_MODE"); devMode == "true" || devMode == "1" {
		config.Development.DevMode = true
	}
	if mockMode := os.Getenv("MOCK_MODE"); mockMode == "true" || mockMode == "1" {
		config.Development.MockMode = true
	}
	if debugMode := os.Getenv("DEBUG_MODE"); debugMode == "true" || debugMode == "1" {
		config.Development.DebugMode = true
	}
}

// LoadForEnvironment loads specific configuration for an environment
func (l *Loader) LoadForEnvironment(environment string) (*Config, error) {
	// Load base config first
	config, err := l.Load()
	if err != nil {
		return nil, err
	}

	// Try to load environment-specific override
	if environment != "" {
		envConfigFile := fmt.Sprintf("config.%s", environment)
		l.v.SetConfigName(envConfigFile)

		if err := l.v.MergeInConfig(); err != nil {
			// Not a critical error if environment file doesn't exist
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to merge environment config: %w", err)
			}
		}

		// Re-unmarshal with merged configuration
		if err := l.v.Unmarshal(config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal merged config: %w", err)
		}

		// Re-apply env var overrides
		l.overrideWithEnvVars(config)
	}

	return config, nil
}

// GetEnvironment determina el entorno actual desde ENV vars
func GetEnvironment() string {
	env := strings.ToLower(os.Getenv("ENV"))
	if env == "" {
		env = strings.ToLower(os.Getenv("ENVIRONMENT"))
	}
	if env == "" {
		env = "development" // Default
	}
	return env
}
