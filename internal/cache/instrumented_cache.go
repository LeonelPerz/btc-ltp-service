package cache

import (
	"time"

	"btc-ltp-service/internal/metrics"
)

// InstrumentedCache wraps any Cache implementation with metrics
type InstrumentedCache struct {
	cache   Cache
	backend string
}

// NewInstrumentedCache creates a new instrumented cache wrapper
func NewInstrumentedCache(cache Cache, backend string) *InstrumentedCache {
	return &InstrumentedCache{
		cache:   cache,
		backend: backend,
	}
}

// Set stores a price for the given trading pair with current timestamp
func (ic *InstrumentedCache) Set(pair string, price float64) error {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "set", time.Since(start))
	}()

	return ic.cache.Set(pair, price)
}

// Get retrieves a cached price if it exists and is still valid
func (ic *InstrumentedCache) Get(pair string) (float64, bool, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "get", time.Since(start))
	}()

	price, found, err := ic.cache.Get(pair)

	if err == nil {
		if found {
			metrics.RecordCacheHit(ic.backend)
		} else {
			metrics.RecordCacheMiss(ic.backend)
		}
	}

	return price, found, err
}

// SetMultiple stores multiple prices atomically
func (ic *InstrumentedCache) SetMultiple(prices map[string]float64) error {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "set_multiple", time.Since(start))
	}()

	return ic.cache.SetMultiple(prices)
}

// GetMultiple retrieves multiple cached prices, returning only valid ones
func (ic *InstrumentedCache) GetMultiple(pairs []string) (map[string]float64, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "get_multiple", time.Since(start))
	}()

	result, err := ic.cache.GetMultiple(pairs)

	if err == nil {
		// Record hits and misses
		for _, pair := range pairs {
			if _, found := result[pair]; found {
				metrics.RecordCacheHit(ic.backend)
			} else {
				metrics.RecordCacheMiss(ic.backend)
			}
		}
	}

	return result, err
}

// IsExpired checks if a cached price for the given pair is expired
func (ic *InstrumentedCache) IsExpired(pair string) (bool, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "is_expired", time.Since(start))
	}()

	return ic.cache.IsExpired(pair)
}

// GetExpiredPairs returns a list of pairs that have expired cache entries
func (ic *InstrumentedCache) GetExpiredPairs(pairs []string) ([]string, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "get_expired_pairs", time.Since(start))
	}()

	return ic.cache.GetExpiredPairs(pairs)
}

// Clear removes all cached prices
func (ic *InstrumentedCache) Clear() error {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "clear", time.Since(start))
	}()

	return ic.cache.Clear()
}

// Size returns the number of cached price entries
func (ic *InstrumentedCache) Size() (int, error) {
	start := time.Now()
	defer func() {
		metrics.RecordCacheOperation(ic.backend, "size", time.Since(start))
	}()

	return ic.cache.Size()
}

// Close closes any connections and cleans up resources
func (ic *InstrumentedCache) Close() error {
	return ic.cache.Close()
}
