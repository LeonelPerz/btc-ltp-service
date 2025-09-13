package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"btc-ltp-service/internal/model"

	"github.com/redis/go-redis/v9"
)

// RedisCache manages cached cryptocurrency prices using Redis
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// NewRedisCache creates a new Redis-backed price cache instance
func NewRedisCache(config CacheConfig) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: rdb,
		ttl:    config.TTL,
		prefix: "btc_ltp:",
	}, nil
}

// Set stores a price for the given trading pair with current timestamp
func (rc *RedisCache) Set(pair string, price float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cachedPrice := model.CachedPrice{
		Price:     price,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(cachedPrice)
	if err != nil {
		return fmt.Errorf("failed to marshal cached price: %w", err)
	}

	key := rc.prefix + pair
	return rc.client.Set(ctx, key, string(data), rc.ttl).Err()
}

// Get retrieves a cached price if it exists and is still valid
func (rc *RedisCache) Get(pair string) (float64, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := rc.prefix + pair
	val, err := rc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, false, nil // Key doesn't exist
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get cached price: %w", err)
	}

	var cachedPrice model.CachedPrice
	if err := json.Unmarshal([]byte(val), &cachedPrice); err != nil {
		return 0, false, fmt.Errorf("failed to unmarshal cached price: %w", err)
	}

	// Check if cached price is still valid (Redis TTL should handle this, but double-check)
	if time.Since(cachedPrice.Timestamp) > rc.ttl {
		return 0, false, nil
	}

	return cachedPrice.Price, true, nil
}

// SetMultiple stores multiple prices atomically using pipeline
func (rc *RedisCache) SetMultiple(prices map[string]float64) error {
	if len(prices) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipe := rc.client.Pipeline()
	now := time.Now()

	for pair, price := range prices {
		cachedPrice := model.CachedPrice{
			Price:     price,
			Timestamp: now,
		}

		data, err := json.Marshal(cachedPrice)
		if err != nil {
			return fmt.Errorf("failed to marshal cached price for %s: %w", pair, err)
		}

		key := rc.prefix + pair
		pipe.Set(ctx, key, string(data), rc.ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetMultiple retrieves multiple cached prices, returning only valid ones
func (rc *RedisCache) GetMultiple(pairs []string) (map[string]float64, error) {
	if len(pairs) == 0 {
		return make(map[string]float64), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create keys for all pairs
	keys := make([]string, len(pairs))
	for i, pair := range pairs {
		keys[i] = rc.prefix + pair
	}

	// Get all values at once using pipeline
	pipe := rc.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get multiple cached prices: %w", err)
	}

	result := make(map[string]float64)
	now := time.Now()

	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err == redis.Nil {
			continue // Key doesn't exist, skip
		}
		if err != nil {
			continue // Error retrieving this key, skip
		}

		var cachedPrice model.CachedPrice
		if err := json.Unmarshal([]byte(val), &cachedPrice); err != nil {
			continue // Error unmarshaling, skip
		}

		// Check if cached price is still valid
		if now.Sub(cachedPrice.Timestamp) <= rc.ttl {
			result[pairs[i]] = cachedPrice.Price
		}
	}

	return result, nil
}

// IsExpired checks if a cached price for the given pair is expired
func (rc *RedisCache) IsExpired(pair string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := rc.prefix + pair
	ttl, err := rc.client.TTL(ctx, key).Result()
	if err != nil {
		return true, fmt.Errorf("failed to check TTL: %w", err)
	}

	// If TTL is -1, key exists but has no expire, consider expired
	// If TTL is -2, key doesn't exist, consider expired
	// If TTL > 0, key exists and has time left
	return ttl <= 0, nil
}

// GetExpiredPairs returns a list of pairs that have expired cache entries
func (rc *RedisCache) GetExpiredPairs(pairs []string) ([]string, error) {
	var expiredPairs []string

	for _, pair := range pairs {
		expired, err := rc.IsExpired(pair)
		if err != nil {
			// If there's an error checking, consider it expired
			expiredPairs = append(expiredPairs, pair)
			continue
		}
		if expired {
			expiredPairs = append(expiredPairs, pair)
		}
	}

	return expiredPairs, nil
}

// Clear removes all cached prices with our prefix
func (rc *RedisCache) Clear() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all keys with our prefix
	keys, err := rc.client.Keys(ctx, rc.prefix+"*").Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for clearing: %w", err)
	}

	if len(keys) == 0 {
		return nil // Nothing to clear
	}

	// Delete all keys
	return rc.client.Del(ctx, keys...).Err()
}

// Size returns the number of cached price entries with our prefix
func (rc *RedisCache) Size() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get all keys with our prefix
	keys, err := rc.client.Keys(ctx, rc.prefix+"*").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get keys for size: %w", err)
	}

	return len(keys), nil
}

// Close closes the Redis connection
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// NewCacheFromConfig creates a cache instance based on configuration
func NewCacheFromConfig(backend string, config CacheConfig) (Cache, error) {
	var cache Cache
	var err error

	switch strings.ToLower(backend) {
	case "memory", "":
		cache = NewPriceCache(config)
	case "redis":
		cache, err = NewRedisCache(config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported cache backend: %s", backend)
	}

	// Wrap with instrumented cache for metrics
	return NewInstrumentedCache(cache, strings.ToLower(backend)), nil
}
