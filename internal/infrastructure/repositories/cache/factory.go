package cache

import (
	"btc-ltp-service/internal/domain/interfaces"
	"btc-ltp-service/internal/infrastructure/logging"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheType represents the type of cache implementation
type CacheType string

const (
	CacheTypeMemory CacheType = "memory"
	CacheTypeRedis  CacheType = "redis"
)

// Config holds cache configuration options
type Config struct {
	Type     CacheType
	RedisURL string
	RedisDB  int
	Password string
}

// Factory provides methods to create cache instances
type Factory struct{}

// NewFactory creates a new cache factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateCache creates a cache instance based on configuration
func (f *Factory) CreateCache(config Config) (interfaces.Cache, error) {
	ctx := context.Background()

	switch config.Type {
	case CacheTypeMemory:
		logging.Info(ctx, "Creating memory cache", logging.Fields{
			"type": "memory",
		})
		return NewMemoryCache(), nil

	case CacheTypeRedis:
		logging.Info(ctx, "Creating Redis cache", logging.Fields{
			"type":     "redis",
			"addr":     config.RedisURL,
			"database": config.RedisDB,
		})
		return f.createRedisCache(config)

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", config.Type)
	}
}

// createRedisCache creates and tests Redis connection
func (f *Factory) createRedisCache(config Config) (interfaces.Cache, error) {
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisURL,
		Password: config.Password,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", config.RedisURL, err)
	}

	logging.Info(context.Background(), "Redis connection established successfully", logging.Fields{
		"addr":     config.RedisURL,
		"database": config.RedisDB,
	})
	return NewRedisCacheWithClient(rdb), nil
}

// CreateCacheFromEnv creates a cache instance from environment variables
func (f *Factory) CreateCacheFromEnv(cacheBackend, redisAddr, redisPassword string, redisDB int) (interfaces.Cache, error) {
	config := Config{
		Type:     CacheType(cacheBackend),
		RedisURL: redisAddr,
		RedisDB:  redisDB,
		Password: redisPassword,
	}

	return f.CreateCache(config)
}
