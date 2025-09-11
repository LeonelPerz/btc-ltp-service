package cache

import (
	"sync"
	"time"

	"btc-ltp-service/internal/model"
)

const (
	// CacheExpiry defines how long cached prices are valid (1 minute for up-to-the-minute accuracy)
	CacheExpiry = 1 * time.Minute
)

// PriceCache manages cached cryptocurrency prices with time-based expiry
type PriceCache struct {
	mutex  sync.RWMutex
	prices map[string]model.CachedPrice
}

// NewPriceCache creates a new price cache instance
func NewPriceCache() *PriceCache {
	return &PriceCache{
		prices: make(map[string]model.CachedPrice),
	}
}

// Set stores a price for the given trading pair with current timestamp
func (pc *PriceCache) Set(pair string, price float64) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.prices[pair] = model.CachedPrice{
		Price:     price,
		Timestamp: time.Now(),
	}
}

// Get retrieves a cached price if it exists and is still valid (within 1 minute)
func (pc *PriceCache) Get(pair string) (float64, bool) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	cachedPrice, exists := pc.prices[pair]
	if !exists {
		return 0, false
	}

	// Check if cached price is still valid (within 1 minute)
	if time.Since(cachedPrice.Timestamp) > CacheExpiry {
		return 0, false
	}

	return cachedPrice.Price, true
}

// GetMultiple retrieves multiple cached prices, returning only valid ones
func (pc *PriceCache) GetMultiple(pairs []string) map[string]float64 {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	result := make(map[string]float64)
	now := time.Now()

	for _, pair := range pairs {
		if cachedPrice, exists := pc.prices[pair]; exists {
			// Only include if cache is still valid
			if now.Sub(cachedPrice.Timestamp) <= CacheExpiry {
				result[pair] = cachedPrice.Price
			}
		}
	}

	return result
}

// SetMultiple stores multiple prices atomically
func (pc *PriceCache) SetMultiple(prices map[string]float64) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	now := time.Now()
	for pair, price := range prices {
		pc.prices[pair] = model.CachedPrice{
			Price:     price,
			Timestamp: now,
		}
	}
}

// IsExpired checks if a cached price for the given pair is expired
func (pc *PriceCache) IsExpired(pair string) bool {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	cachedPrice, exists := pc.prices[pair]
	if !exists {
		return true
	}

	return time.Since(cachedPrice.Timestamp) > CacheExpiry
}

// GetExpiredPairs returns a list of pairs that have expired cache entries
func (pc *PriceCache) GetExpiredPairs(pairs []string) []string {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	var expiredPairs []string
	now := time.Now()

	for _, pair := range pairs {
		if cachedPrice, exists := pc.prices[pair]; !exists || now.Sub(cachedPrice.Timestamp) > CacheExpiry {
			expiredPairs = append(expiredPairs, pair)
		}
	}

	return expiredPairs
}

// Clear removes all cached prices
func (pc *PriceCache) Clear() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.prices = make(map[string]model.CachedPrice)
}

// Size returns the number of cached price entries
func (pc *PriceCache) Size() int {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	return len(pc.prices)
}
