package cache

import (
	"btc-ltp-service/internal/domain/interfaces"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr, password string, db int) interfaces.Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisCache{
		client: rdb,
	}
}

// NewRedisCacheWithClient creates a new Redis cache instance with an existing client
func NewRedisCacheWithClient(client *redis.Client) interfaces.Cache {
	return &RedisCache{
		client: client,
	}
}

// Get retrieves a value from Redis
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Set stores a value in Redis with TTL
func (r *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Delete removes a key from Redis
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Ping checks if Redis connection is alive
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Size returns the number of keys in Redis (for debugging)
func (r *RedisCache) Size(ctx context.Context) (int64, error) {
	return r.client.DBSize(ctx).Result()
}

// FlushAll removes all keys from Redis (for testing)
func (r *RedisCache) FlushAll(ctx context.Context) error {
	return r.client.FlushAll(ctx).Err()
}
