package cache

import "time"

// Cache interface defines the contract for different cache implementations
type Cache interface {
	// Set stores a price for the given trading pair with current timestamp
	Set(pair string, price float64) error

	// Get retrieves a cached price if it exists and is still valid
	Get(pair string) (float64, bool, error)

	// SetMultiple stores multiple prices atomically
	SetMultiple(prices map[string]float64) error

	// GetMultiple retrieves multiple cached prices, returning only valid ones
	GetMultiple(pairs []string) (map[string]float64, error)

	// IsExpired checks if a cached price for the given pair is expired
	IsExpired(pair string) (bool, error)

	// GetExpiredPairs returns a list of pairs that have expired cache entries
	GetExpiredPairs(pairs []string) ([]string, error)

	// Clear removes all cached prices
	Clear() error

	// Size returns the number of cached price entries
	Size() (int, error)

	// Close closes any connections and cleans up resources
	Close() error
}

// CacheConfig holds configuration for cache implementations
type CacheConfig struct {
	TTL           time.Duration
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}
