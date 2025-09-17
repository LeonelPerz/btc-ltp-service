package cache

import (
	"btc-ltp-service/internal/domain/interfaces"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
}

func TestFactory_CreateCache(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		wantType    string
		expectedErr string
	}{
		{
			name: "memory cache type",
			config: Config{
				Type: CacheTypeMemory,
			},
			wantErr:  false,
			wantType: "*cache.MemoryCache",
		},
		{
			name: "redis cache type - valid config",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  0,
				Password: "",
			},
			wantErr:  false, // Puede pasar si hay Redis corriendo
			wantType: "*cache.RedisCache",
		},
		{
			name: "redis cache type - with password",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  0,
				Password: "secret",
			},
			wantErr:  false, // Puede pasar si hay Redis corriendo
			wantType: "*cache.RedisCache",
		},
		{
			name: "redis cache type - different database",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  1,
				Password: "",
			},
			wantErr:  false, // Puede pasar si hay Redis corriendo
			wantType: "*cache.RedisCache",
		},
		{
			name: "unsupported cache type",
			config: Config{
				Type: "unsupported",
			},
			wantErr:     true,
			expectedErr: "unsupported cache type",
		},
		{
			name: "empty config",
			config: Config{
				Type: "",
			},
			wantErr:     true,
			expectedErr: "unsupported cache type",
		},
		{
			name: "redis with invalid URL",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "invalid-url",
				RedisDB:  0,
				Password: "",
			},
			wantErr:     true,
			expectedErr: "failed to connect to Redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cache, err := factory.CreateCache(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cache)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cache)

				// Verificar el tipo de cache creado
				if tt.wantType != "" {
					switch tt.wantType {
					case "*cache.MemoryCache":
						_, ok := cache.(*MemoryCache)
						assert.True(t, ok, "Expected MemoryCache, got %T", cache)
					case "*cache.RedisCache":
						_, ok := cache.(*RedisCache)
						assert.True(t, ok, "Expected RedisCache, got %T", cache)
					}
				}

				// Verificar que implementa la interfaz Cache
				_, ok := cache.(interfaces.Cache)
				assert.True(t, ok, "Cache should implement interfaces.Cache")
			}
		})
	}
}

func TestFactory_CreateCacheFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		cacheBackend  string
		redisAddr     string
		redisPassword string
		redisDB       int
		wantErr       bool
		wantType      string
		expectedErr   string
	}{
		{
			name:          "memory backend from env",
			cacheBackend:  "memory",
			redisAddr:     "",
			redisPassword: "",
			redisDB:       0,
			wantErr:       false,
			wantType:      "*cache.MemoryCache",
		},
		{
			name:          "redis backend from env",
			cacheBackend:  "redis",
			redisAddr:     "localhost:6379",
			redisPassword: "",
			redisDB:       0,
			wantErr:       false, // Puede pasar si hay Redis corriendo
			wantType:      "*cache.RedisCache",
		},
		{
			name:          "redis backend with password from env",
			cacheBackend:  "redis",
			redisAddr:     "localhost:6379",
			redisPassword: "secret",
			redisDB:       0,
			wantErr:       false, // Puede pasar si hay Redis corriendo
			wantType:      "*cache.RedisCache",
		},
		{
			name:          "redis backend different DB from env",
			cacheBackend:  "redis",
			redisAddr:     "localhost:6379",
			redisPassword: "",
			redisDB:       2,
			wantErr:       false, // Puede pasar si hay Redis corriendo
			wantType:      "*cache.RedisCache",
		},
		{
			name:          "invalid backend from env",
			cacheBackend:  "invalid",
			redisAddr:     "",
			redisPassword: "",
			redisDB:       0,
			wantErr:       true,
			expectedErr:   "unsupported cache type",
		},
		{
			name:          "empty parameters",
			cacheBackend:  "",
			redisAddr:     "",
			redisPassword: "",
			redisDB:       0,
			wantErr:       true,
			expectedErr:   "unsupported cache type",
		},
		{
			name:          "redis connection failure from env",
			cacheBackend:  "redis",
			redisAddr:     "invalid-host:6379",
			redisPassword: "",
			redisDB:       0,
			wantErr:       true,
			expectedErr:   "failed to connect to Redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cache, err := factory.CreateCacheFromEnv(
				tt.cacheBackend,
				tt.redisAddr,
				tt.redisPassword,
				tt.redisDB,
			)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cache)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cache)

				// Verificar el tipo de cache creado
				if tt.wantType != "" {
					switch tt.wantType {
					case "*cache.MemoryCache":
						_, ok := cache.(*MemoryCache)
						assert.True(t, ok, "Expected MemoryCache, got %T", cache)
					case "*cache.RedisCache":
						_, ok := cache.(*RedisCache)
						assert.True(t, ok, "Expected RedisCache, got %T", cache)
					}
				}

				// Verificar que implementa la interfaz Cache
				_, ok := cache.(interfaces.Cache)
				assert.True(t, ok, "Cache should implement interfaces.Cache")
			}
		})
	}
}

func TestCacheType_Constants(t *testing.T) {
	tests := []struct {
		name      string
		cacheType CacheType
		expected  string
	}{
		{
			name:      "memory cache type",
			cacheType: CacheTypeMemory,
			expected:  "memory",
		},
		{
			name:      "redis cache type",
			cacheType: CacheTypeRedis,
			expected:  "redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.cacheType))
		})
	}
}

func TestConfig_Struct(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "memory config",
			config: Config{
				Type:     CacheTypeMemory,
				RedisURL: "",
				RedisDB:  0,
				Password: "",
			},
		},
		{
			name: "redis config with all fields",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  1,
				Password: "secret",
			},
		},
		{
			name: "redis config minimal",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  0,
				Password: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verificar que la estructura se puede crear correctamente
			config := tt.config
			assert.NotNil(t, config)

			// Verificar campos específicos
			switch config.Type {
			case CacheTypeMemory:
				assert.Equal(t, CacheTypeMemory, config.Type)
			case CacheTypeRedis:
				assert.Equal(t, CacheTypeRedis, config.Type)
				assert.NotEmpty(t, config.RedisURL)
				assert.GreaterOrEqual(t, config.RedisDB, 0)
			}
		})
	}
}

// Test de integración básica para verificar que los caches creados funcionan
func TestFactory_Integration_BasicFunctionality(t *testing.T) {
	factory := NewFactory()

	t.Run("memory cache basic operations", func(t *testing.T) {
		config := Config{
			Type: CacheTypeMemory,
		}

		cache, err := factory.CreateCache(config)
		assert.NoError(t, err)
		assert.NotNil(t, cache)

		// Test basic operations
		ctx := context.Background()

		// Set operation
		err = cache.Set(ctx, "test-key", "test-value", 5*time.Minute)
		assert.NoError(t, err)

		// Get operation
		value, err := cache.Get(ctx, "test-key")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", value)

		// Delete operation
		err = cache.Delete(ctx, "test-key")
		assert.NoError(t, err)

		// Verify deletion
		value, err = cache.Get(ctx, "test-key")
		assert.Error(t, err)
		assert.Equal(t, "", value)
		assert.Equal(t, ErrKeyNotFound, err)
	})
}

// Tests para casos edge con configuraciones límite
func TestFactory_EdgeCases(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "redis with very high DB number",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  15, // Redis default max DB is 15
				Password: "",
			},
			wantErr: false, // Puede pasar si hay Redis corriendo
		},
		{
			name: "redis with negative DB number",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  -1,
				Password: "",
			},
			wantErr: false, // Redis acepta DB negativos
		},
		{
			name: "redis with complex password",
			config: Config{
				Type:     CacheTypeRedis,
				RedisURL: "localhost:6379",
				RedisDB:  0,
				Password: "c0mp1ex-p@ssw0rd!#$%",
			},
			wantErr: false, // Puede pasar si hay Redis corriendo
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := factory.CreateCache(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cache)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cache)
			}
		})
	}
}
