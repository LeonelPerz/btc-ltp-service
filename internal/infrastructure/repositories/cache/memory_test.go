package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMemoryCache(t *testing.T) {
	tests := []struct {
		name string
		want bool // true if we want a non-nil cache
	}{
		{
			name: "creates valid memory cache",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache()
			if tt.want {
				assert.NotNil(t, cache)
				// Verificar que implementa la interfaz
				memCache, ok := cache.(*MemoryCache)
				assert.True(t, ok)
				assert.NotNil(t, memCache.items)
				assert.Equal(t, 0, len(memCache.items))
			} else {
				assert.Nil(t, cache)
			}
		})
	}
}

func TestMemoryCache_Set(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		ttl      time.Duration
		wantErr  bool
		validate func(*testing.T, *MemoryCache)
	}{
		{
			name:    "valid key-value",
			key:     "test-key",
			value:   "test-value",
			ttl:     5 * time.Minute,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				assert.Equal(t, 1, cache.Size())
			},
		},
		{
			name:    "empty key",
			key:     "",
			value:   "test-value",
			ttl:     5 * time.Minute,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				assert.Equal(t, 1, cache.Size())
			},
		},
		{
			name:    "empty value",
			key:     "test-key",
			value:   "",
			ttl:     5 * time.Minute,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				assert.Equal(t, 1, cache.Size())
			},
		},
		{
			name:    "zero TTL",
			key:     "test-key",
			value:   "test-value",
			ttl:     0,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				// Con TTL 0, el item expira inmediatamente
				time.Sleep(1 * time.Millisecond)
				val, err := cache.Get(context.Background(), "test-key")
				assert.Equal(t, "", val)
				assert.Equal(t, ErrKeyExpired, err)
			},
		},
		{
			name:    "negative TTL",
			key:     "test-key",
			value:   "test-value",
			ttl:     -1 * time.Minute,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				// Con TTL negativo, el item ya está expirado
				val, err := cache.Get(context.Background(), "test-key")
				assert.Equal(t, "", val)
				assert.Equal(t, ErrKeyExpired, err)
			},
		},
		{
			name:    "very long TTL",
			key:     "test-key",
			value:   "test-value",
			ttl:     24 * time.Hour,
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				val, err := cache.Get(context.Background(), "test-key")
				assert.NoError(t, err)
				assert.Equal(t, "test-value", val)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			ctx := context.Background()

			err := cache.Set(ctx, tt.key, tt.value, tt.ttl)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, cache)
			}
		})
	}
}

func TestMemoryCache_Set_OverwriteExisting(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	// Set initial value
	err := cache.Set(ctx, "key1", "value1", 5*time.Minute)
	assert.NoError(t, err)

	// Verify initial value
	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Overwrite with new value
	err = cache.Set(ctx, "key1", "value2", 10*time.Minute)
	assert.NoError(t, err)

	// Verify overwritten value
	val, err = cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value2", val)

	assert.Equal(t, 1, cache.Size()) // Still only one item
}

func TestMemoryCache_Get(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*MemoryCache)
		key       string
		wantValue string
		wantErr   error
	}{
		{
			name: "existing key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "existing-key", "existing-value", 5*time.Minute)
			},
			key:       "existing-key",
			wantValue: "existing-value",
			wantErr:   nil,
		},
		{
			name:      "non-existent key",
			setupData: func(cache *MemoryCache) {},
			key:       "non-existent",
			wantValue: "",
			wantErr:   ErrKeyNotFound,
		},
		{
			name: "expired key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "expired-key", "expired-value", 1*time.Nanosecond)
				time.Sleep(2 * time.Nanosecond) // Asegurar expiración
			},
			key:       "expired-key",
			wantValue: "",
			wantErr:   ErrKeyExpired,
		},
		{
			name: "empty key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "", "empty-key-value", 5*time.Minute)
			},
			key:       "",
			wantValue: "empty-key-value",
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			tt.setupData(cache)
			ctx := context.Background()

			value, err := cache.Get(ctx, tt.key)

			assert.Equal(t, tt.wantValue, value)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*MemoryCache)
		key       string
		wantErr   bool
		validate  func(*testing.T, *MemoryCache)
	}{
		{
			name: "existing key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "key-to-delete", "value", 5*time.Minute)
			},
			key:     "key-to-delete",
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				val, err := cache.Get(context.Background(), "key-to-delete")
				assert.Equal(t, "", val)
				assert.Equal(t, ErrKeyNotFound, err)
				assert.Equal(t, 0, cache.Size())
			},
		},
		{
			name:      "non-existent key",
			setupData: func(cache *MemoryCache) {},
			key:       "non-existent",
			wantErr:   false, // Delete no falla si la clave no existe
			validate: func(t *testing.T, cache *MemoryCache) {
				assert.Equal(t, 0, cache.Size())
			},
		},
		{
			name: "empty key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "", "empty-key-value", 5*time.Minute)
			},
			key:     "",
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				val, err := cache.Get(context.Background(), "")
				assert.Equal(t, "", val)
				assert.Equal(t, ErrKeyNotFound, err)
			},
		},
		{
			name: "expired key",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "expired", "value", 1*time.Nanosecond)
				time.Sleep(2 * time.Nanosecond)
			},
			key:     "expired",
			wantErr: false,
			validate: func(t *testing.T, cache *MemoryCache) {
				assert.Equal(t, 0, cache.Size())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			tt.setupData(cache)
			ctx := context.Background()

			err := cache.Delete(ctx, tt.key)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, cache)
			}
		})
	}
}

func TestMemoryCache_Size(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*MemoryCache)
		wantSize  int
	}{
		{
			name:      "empty cache",
			setupData: func(cache *MemoryCache) {},
			wantSize:  0,
		},
		{
			name: "with items",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "key1", "value1", 5*time.Minute)
				_ = cache.Set(context.Background(), "key2", "value2", 5*time.Minute)
				_ = cache.Set(context.Background(), "key3", "value3", 5*time.Minute)
			},
			wantSize: 3,
		},
		{
			name: "after expiration",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "key1", "value1", 5*time.Minute)
				_ = cache.Set(context.Background(), "key2", "value2", 1*time.Nanosecond)
				time.Sleep(2 * time.Nanosecond)
			},
			wantSize: 2, // Expired items are still counted until cleanup
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			tt.setupData(cache)

			size := cache.Size()
			assert.Equal(t, tt.wantSize, size)
		})
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*MemoryCache)
		wantSize  int
	}{
		{
			name:      "empty cache",
			setupData: func(cache *MemoryCache) {},
			wantSize:  0,
		},
		{
			name: "remove expired items",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "valid1", "value1", 5*time.Minute)
				_ = cache.Set(context.Background(), "expired1", "value2", 1*time.Nanosecond)
				_ = cache.Set(context.Background(), "expired2", "value3", 1*time.Nanosecond)
				_ = cache.Set(context.Background(), "valid2", "value4", 5*time.Minute)
				time.Sleep(2 * time.Nanosecond)
			},
			wantSize: 2, // Solo los válidos deben quedar
		},
		{
			name: "keep valid items",
			setupData: func(cache *MemoryCache) {
				_ = cache.Set(context.Background(), "valid1", "value1", 5*time.Minute)
				_ = cache.Set(context.Background(), "valid2", "value2", 10*time.Minute)
			},
			wantSize: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			tt.setupData(cache)

			cache.Cleanup()

			size := cache.Size()
			assert.Equal(t, tt.wantSize, size)
		})
	}
}

func TestMemoryCache_AutoCleanupOnSet(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	// Agregar algunos elementos que expiran rápidamente
	_ = cache.Set(ctx, "expired1", "value1", 1*time.Nanosecond)
	_ = cache.Set(ctx, "expired2", "value2", 1*time.Nanosecond)
	_ = cache.Set(ctx, "valid", "value3", 5*time.Minute)

	// Esperar a que expiren
	time.Sleep(2 * time.Nanosecond)

	initialSize := int(cache.Size())
	assert.Equal(t, 1, initialSize)

	// Set un nuevo elemento, esto debería triggerar la limpieza
	_ = cache.Set(ctx, "new", "new-value", 5*time.Minute)

	// Verificar que los elementos expirados fueron eliminados
	finalSize := int(cache.Size())
	assert.Equal(t, 2, finalSize)
}

func TestMemoryCache_Expiration(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		waitTime time.Duration
		wantErr  error
	}{
		{
			name:     "immediate expiry",
			ttl:      0,
			waitTime: 1 * time.Millisecond,
			wantErr:  ErrKeyExpired,
		},
		{
			name:     "delayed expiry",
			ttl:      10 * time.Millisecond,
			waitTime: 15 * time.Millisecond,
			wantErr:  ErrKeyExpired,
		},
		{
			name:     "no expiry",
			ttl:      1 * time.Hour,
			waitTime: 1 * time.Millisecond,
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			ctx := context.Background()

			err := cache.Set(ctx, "test-key", "test-value", tt.ttl)
			assert.NoError(t, err)

			time.Sleep(tt.waitTime)

			value, err := cache.Get(ctx, "test-key")

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
				assert.Equal(t, "", value)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "test-value", value)
			}
		})
	}
}

func TestMemoryCache_Concurrency(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	t.Run("concurrent writes", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("key-%d", id)
				value := fmt.Sprintf("value-%d", id)
				err := cache.Set(ctx, key, value, 5*time.Minute)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, numGoroutines, cache.Size())
	})

	t.Run("concurrent reads", func(t *testing.T) {
		// Setup some data
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("read-key-%d", i)
			value := fmt.Sprintf("read-value-%d", i)
			_ = cache.Set(ctx, key, value, 5*time.Minute)
		}

		var wg sync.WaitGroup
		numGoroutines := 50

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("read-key-%d", id%10)
				expectedValue := fmt.Sprintf("read-value-%d", id%10)

				value, err := cache.Get(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, expectedValue, value)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent read-write", func(t *testing.T) {
		var wg sync.WaitGroup
		numOperations := 100

		for i := 0; i < numOperations; i++ {
			wg.Add(2) // Una lectura y una escritura

			// Escritura
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("rw-key-%d", id%10)
				value := fmt.Sprintf("rw-value-%d", id)
				_ = cache.Set(ctx, key, value, 5*time.Minute)
			}(i)

			// Lectura
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("rw-key-%d", id%10)
				_, _ = cache.Get(ctx, key) // Ignorar resultado
			}(i)
		}

		wg.Wait()
		// No hay assertions específicas, solo verificamos que no haya race conditions
	})

	t.Run("concurrent deletes", func(t *testing.T) {
		// Setup data
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("delete-key-%d", i)
			value := fmt.Sprintf("delete-value-%d", i)
			_ = cache.Set(ctx, key, value, 5*time.Minute)
		}

		var wg sync.WaitGroup
		numGoroutines := 50

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("delete-key-%d", id)
				err := cache.Delete(ctx, key)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
		// El cache puede tener elementos de otros tests concurrentes
		// Verificamos que al menos los elementos que agregamos fueron eliminados
		finalSize := cache.Size()
		assert.LessOrEqual(t, finalSize, 120) // Puede haber elementos de otros tests
	})
}

func TestMemoryCache_EdgeCases(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	t.Run("very short TTL", func(t *testing.T) {
		err := cache.Set(ctx, "short-ttl", "value", 1*time.Nanosecond)
		assert.NoError(t, err)

		time.Sleep(2 * time.Nanosecond)

		value, err := cache.Get(ctx, "short-ttl")
		assert.Equal(t, ErrKeyExpired, err)
		assert.Equal(t, "", value)
	})

	t.Run("very long TTL", func(t *testing.T) {
		err := cache.Set(ctx, "long-ttl", "value", 100*24*time.Hour) // 100 días
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "long-ttl")
		assert.NoError(t, err)
		assert.Equal(t, "value", value)
	})

	t.Run("unicode handling", func(t *testing.T) {
		unicodeKey := "🔑-key"
		unicodeValue := "🎯-value-ñáéíóú"

		err := cache.Set(ctx, unicodeKey, unicodeValue, 5*time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, unicodeKey)
		assert.NoError(t, err)
		assert.Equal(t, unicodeValue, value)
	})

	t.Run("large value", func(t *testing.T) {
		largeValue := make([]byte, 1024*1024) // 1MB
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		err := cache.Set(ctx, "large-key", string(largeValue), 5*time.Minute)
		assert.NoError(t, err)

		value, err := cache.Get(ctx, "large-key")
		assert.NoError(t, err)
		assert.Equal(t, string(largeValue), value)
	})
}
