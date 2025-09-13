package cache

import (
	"sync"
	"time"

	"btc-ltp-service/internal/model"
)

// PriceCache manages cached cryptocurrency prices with time-based expiry (in-memory)
type PriceCache struct {
	mutex  sync.RWMutex
	prices map[string]model.CachedPrice
	ttl    time.Duration
}

// NewPriceCache creates a new in-memory price cache instance
func NewPriceCache(config CacheConfig) *PriceCache {
	return &PriceCache{
		prices: make(map[string]model.CachedPrice),
		ttl:    config.TTL,
	}
}

// Set stores a price for the given trading pair with current timestamp
func (pc *PriceCache) Set(pair string, price float64) error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.prices[pair] = model.CachedPrice{
		Price:     price,
		Timestamp: time.Now(),
	}
	return nil
}

// Get retrieves a cached price if it exists and is still valid
func (pc *PriceCache) Get(pair string) (float64, bool, error) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	cachedPrice, exists := pc.prices[pair]
	if !exists {
		return 0, false, nil
	}

	// Check if cached price is still valid
	if time.Since(cachedPrice.Timestamp) > pc.ttl {
		return 0, false, nil
	}

	return cachedPrice.Price, true, nil
}

// GetMultiple retrieves multiple cached prices, returning only valid ones
func (pc *PriceCache) GetMultiple(pairs []string) (map[string]float64, error) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	result := make(map[string]float64)
	now := time.Now()

	for _, pair := range pairs {
		if cachedPrice, exists := pc.prices[pair]; exists {
			// Only include if cache is still valid
			if now.Sub(cachedPrice.Timestamp) <= pc.ttl {
				result[pair] = cachedPrice.Price
			}
		}
	}

	return result, nil
}

// SetMultiple stores multiple prices atomically
func (pc *PriceCache) SetMultiple(prices map[string]float64) error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	now := time.Now()
	for pair, price := range prices {
		pc.prices[pair] = model.CachedPrice{
			Price:     price,
			Timestamp: now,
		}
	}
	return nil
}

// IsExpired checks if a cached price for the given pair is expired
func (pc *PriceCache) IsExpired(pair string) (bool, error) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	cachedPrice, exists := pc.prices[pair]
	if !exists {
		return true, nil
	}

	return time.Since(cachedPrice.Timestamp) > pc.ttl, nil
}

// GetExpiredPairs returns a list of pairs that have expired cache entries
func (pc *PriceCache) GetExpiredPairs(pairs []string) ([]string, error) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	var expiredPairs []string
	now := time.Now()

	for _, pair := range pairs {
		if cachedPrice, exists := pc.prices[pair]; !exists || now.Sub(cachedPrice.Timestamp) > pc.ttl {
			expiredPairs = append(expiredPairs, pair)
		}
	}

	return expiredPairs, nil
}

// Clear removes all cached prices
func (pc *PriceCache) Clear() error {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.prices = make(map[string]model.CachedPrice)
	return nil
}

// Size returns the number of cached price entries
func (pc *PriceCache) Size() (int, error) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	return len(pc.prices), nil
}

// Close closes any connections and cleans up resources (no-op for in-memory)
func (pc *PriceCache) Close() error {
	return nil
}
