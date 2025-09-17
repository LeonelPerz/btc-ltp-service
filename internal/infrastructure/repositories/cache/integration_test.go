package cache

import (
	"btc-ltp-service/internal/domain/entities"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_PriceAdapterWithMemoryCache(t *testing.T) {
	// Setup
	memoryCache := NewMemoryCache()
	adapter := NewPriceCache(memoryCache, 5*time.Minute)
	ctx := context.Background()

	// Test data
	btcPrice := &entities.Price{
		Pair:      "BTC/USD",
		Amount:    50000.0,
		Timestamp: time.Now(),
		Age:       time.Minute,
	}

	ethPrice := &entities.Price{
		Pair:      "ETH/USD",
		Amount:    3000.0,
		Timestamp: time.Now(),
		Age:       30 * time.Second,
	}

	t.Run("complete price lifecycle", func(t *testing.T) {
		// 1. Verify price doesn't exist initially
		price, found := adapter.Get(ctx, "BTC/USD")
		assert.False(t, found)
		assert.Nil(t, price)

		// 2. Set price
		err := adapter.Set(ctx, btcPrice)
		assert.NoError(t, err)

		// 3. Get price back
		retrievedPrice, found := adapter.Get(ctx, "BTC/USD")
		assert.True(t, found)
		assert.NotNil(t, retrievedPrice)
		assert.Equal(t, btcPrice.Pair, retrievedPrice.Pair)
		assert.Equal(t, btcPrice.Amount, retrievedPrice.Amount)

		// 4. Update price
		updatedPrice := &entities.Price{
			Pair:      "BTC/USD",
			Amount:    51000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}
		err = adapter.Set(ctx, updatedPrice)
		assert.NoError(t, err)

		// 5. Verify update
		retrievedPrice, found = adapter.Get(ctx, "BTC/USD")
		assert.True(t, found)
		assert.Equal(t, 51000.0, retrievedPrice.Amount)
	})

	t.Run("multiple prices management", func(t *testing.T) {
		// Set multiple prices
		err := adapter.Set(ctx, btcPrice)
		assert.NoError(t, err)

		err = adapter.Set(ctx, ethPrice)
		assert.NoError(t, err)

		// Test GetMany with all existing
		prices, missing := adapter.GetMany(ctx, []string{"BTC/USD", "ETH/USD"})
		assert.Len(t, prices, 2)
		assert.Len(t, missing, 0)

		// Test GetMany with mixed
		prices, missing = adapter.GetMany(ctx, []string{"BTC/USD", "ADA/USD", "ETH/USD"})
		assert.Len(t, prices, 2)
		assert.Len(t, missing, 1)
		assert.Contains(t, missing, "ADA/USD")

		// Verify order preservation
		pairs := []string{"ETH/USD", "BTC/USD"}
		prices, missing = adapter.GetMany(ctx, pairs)
		assert.Len(t, prices, 2)
		assert.Equal(t, "ETH/USD", prices[0].Pair)
		assert.Equal(t, "BTC/USD", prices[1].Pair)
	})

	t.Run("TTL behavior", func(t *testing.T) {
		// Create adapter with short TTL
		shortTTLAdapter := NewPriceCache(memoryCache, 10*time.Millisecond)

		// Set price with short TTL
		shortLivedPrice := &entities.Price{
			Pair:      "SHORT/USD",
			Amount:    100.0,
			Timestamp: time.Now(),
			Age:       time.Second,
		}

		err := shortTTLAdapter.Set(ctx, shortLivedPrice)
		assert.NoError(t, err)

		// Verify it exists immediately
		price, found := shortTTLAdapter.Get(ctx, "SHORT/USD")
		assert.True(t, found)
		assert.NotNil(t, price)

		// Wait for expiration
		time.Sleep(20 * time.Millisecond)

		// Verify it's expired
		price, found = shortTTLAdapter.Get(ctx, "SHORT/USD")
		assert.False(t, found)
		assert.Nil(t, price)
	})
}

func TestIntegration_CacheFactoryIntegration(t *testing.T) {
	factory := NewFactory()

	t.Run("memory cache through factory", func(t *testing.T) {
		config := Config{
			Type: CacheTypeMemory,
		}

		cache, err := factory.CreateCache(config)
		require.NoError(t, err)
		require.NotNil(t, cache)

		// Create price adapter
		adapter := NewPriceCache(cache, 5*time.Minute)
		ctx := context.Background()

		// Test basic operations
		price := &entities.Price{
			Pair:      "FACTORY/USD",
			Amount:    1000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}

		err = adapter.Set(ctx, price)
		assert.NoError(t, err)

		retrievedPrice, found := adapter.Get(ctx, "FACTORY/USD")
		assert.True(t, found)
		assert.Equal(t, price.Amount, retrievedPrice.Amount)
	})

	t.Run("cache from environment variables", func(t *testing.T) {
		cache, err := factory.CreateCacheFromEnv("memory", "", "", 0)
		require.NoError(t, err)
		require.NotNil(t, cache)

		adapter := NewPriceCache(cache, 10*time.Minute)
		ctx := context.Background()

		// Test operations
		price := &entities.Price{
			Pair:      "ENV/USD",
			Amount:    2000.0,
			Timestamp: time.Now(),
			Age:       2 * time.Minute,
		}

		err = adapter.Set(ctx, price)
		assert.NoError(t, err)

		retrievedPrice, found := adapter.Get(ctx, "ENV/USD")
		assert.True(t, found)
		assert.Equal(t, price.Amount, retrievedPrice.Amount)
	})
}

func TestIntegration_ConcurrentOperations(t *testing.T) {
	memoryCache := NewMemoryCache()
	adapter := NewPriceCache(memoryCache, 10*time.Minute)
	ctx := context.Background()

	t.Run("concurrent writes", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 50

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				price := &entities.Price{
					Pair:      fmt.Sprintf("PAIR%d/USD", id),
					Amount:    float64(1000 + id),
					Timestamp: time.Now(),
					Age:       time.Minute,
				}

				err := adapter.Set(ctx, price)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all prices were set
		for i := 0; i < numGoroutines; i++ {
			pair := fmt.Sprintf("PAIR%d/USD", i)
			price, found := adapter.Get(ctx, pair)
			assert.True(t, found, "Price for %s should exist", pair)
			assert.Equal(t, float64(1000+i), price.Amount)
		}
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		// Setup initial data
		initialPrice := &entities.Price{
			Pair:      "CONCURRENT/USD",
			Amount:    5000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}
		err := adapter.Set(ctx, initialPrice)
		require.NoError(t, err)

		var wg sync.WaitGroup
		numReaders := 25
		numWriters := 25

		// Start readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					price, found := adapter.Get(ctx, "CONCURRENT/USD")
					if found {
						assert.NotNil(t, price)
						assert.Equal(t, "CONCURRENT/USD", price.Pair)
						assert.Greater(t, price.Amount, 0.0)
					}
				}
			}(i)
		}

		// Start writers
		for i := 0; i < numWriters; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < 5; j++ {
					price := &entities.Price{
						Pair:      "CONCURRENT/USD",
						Amount:    float64(5000 + id*10 + j),
						Timestamp: time.Now(),
						Age:       time.Minute,
					}

					err := adapter.Set(ctx, price)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()

		// Verify final state
		finalPrice, found := adapter.Get(ctx, "CONCURRENT/USD")
		assert.True(t, found)
		assert.NotNil(t, finalPrice)
	})

	t.Run("concurrent GetMany operations", func(t *testing.T) {
		// Setup test data
		testPairs := []string{"MULTI1/USD", "MULTI2/USD", "MULTI3/USD", "MULTI4/USD", "MULTI5/USD"}

		for i, pair := range testPairs {
			price := &entities.Price{
				Pair:      pair,
				Amount:    float64(1000 * (i + 1)),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create separate slices for each goroutine to avoid race conditions
				var pairs []string
				if id%2 == 0 {
					// Create a copy to avoid race condition
					pairs = make([]string, len(testPairs[:3]))
					copy(pairs, testPairs[:3])
					pairs = append(pairs, "NONEXISTENT/USD") // Add non-existent pair
				} else {
					// Create a copy to avoid race condition
					pairs = make([]string, len(testPairs[:3]))
					copy(pairs, testPairs[:3])
				}

				prices, missing := adapter.GetMany(ctx, pairs)

				if id%2 == 0 {
					assert.Len(t, prices, 3)
					assert.Len(t, missing, 1)
					assert.Contains(t, missing, "NONEXISTENT/USD")
				} else {
					assert.Len(t, prices, 3)
					assert.Len(t, missing, 0)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestIntegration_ErrorHandling(t *testing.T) {
	memoryCache := NewMemoryCache()
	adapter := NewPriceCache(memoryCache, 5*time.Minute)
	ctx := context.Background()

	t.Run("error propagation", func(t *testing.T) {
		// Test getting non-existent key
		price, found := adapter.Get(ctx, "NONEXISTENT/USD")
		assert.False(t, found)
		assert.Nil(t, price)

		// Test GetMany with all non-existent keys
		prices, missing := adapter.GetMany(ctx, []string{"NONE1/USD", "NONE2/USD"})
		assert.Len(t, prices, 0)
		assert.Len(t, missing, 2)
		assert.Contains(t, missing, "NONE1/USD")
		assert.Contains(t, missing, "NONE2/USD")
	})

	t.Run("partial failures in GetMany", func(t *testing.T) {
		// Set one price
		validPrice := &entities.Price{
			Pair:      "VALID/USD",
			Amount:    1000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}
		err := adapter.Set(ctx, validPrice)
		require.NoError(t, err)

		// Get mix of valid and invalid
		prices, missing := adapter.GetMany(ctx, []string{"VALID/USD", "INVALID1/USD", "INVALID2/USD"})

		assert.Len(t, prices, 1)
		assert.Len(t, missing, 2)
		assert.Equal(t, "VALID/USD", prices[0].Pair)
		assert.Contains(t, missing, "INVALID1/USD")
		assert.Contains(t, missing, "INVALID2/USD")
	})
}

func TestIntegration_TTLBehavior(t *testing.T) {
	memoryCache := NewMemoryCache()

	t.Run("different TTL values", func(t *testing.T) {
		// Short TTL adapter
		shortAdapter := NewPriceCache(memoryCache, 50*time.Millisecond)
		// Long TTL adapter
		longAdapter := NewPriceCache(memoryCache, 10*time.Minute)

		ctx := context.Background()

		shortPrice := &entities.Price{
			Pair:      "SHORT/USD",
			Amount:    1000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}

		longPrice := &entities.Price{
			Pair:      "LONG/USD",
			Amount:    2000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}

		// Set both prices
		err := shortAdapter.Set(ctx, shortPrice)
		require.NoError(t, err)

		err = longAdapter.Set(ctx, longPrice)
		require.NoError(t, err)

		// Both should exist initially
		price, found := shortAdapter.Get(ctx, "SHORT/USD")
		assert.True(t, found)
		assert.NotNil(t, price)

		price, found = longAdapter.Get(ctx, "LONG/USD")
		assert.True(t, found)
		assert.NotNil(t, price)

		// Wait for short TTL to expire
		time.Sleep(100 * time.Millisecond)

		// Short should be expired, long should still exist
		price, found = shortAdapter.Get(ctx, "SHORT/USD")
		assert.False(t, found)
		assert.Nil(t, price)

		price, found = longAdapter.Get(ctx, "LONG/USD")
		assert.True(t, found)
		assert.NotNil(t, price)
	})

	t.Run("TTL impact on GetMany", func(t *testing.T) {
		adapter := NewPriceCache(memoryCache, 30*time.Millisecond)
		ctx := context.Background()

		// Set multiple prices
		pairs := []string{"TTL1/USD", "TTL2/USD", "TTL3/USD"}
		for i, pair := range pairs {
			price := &entities.Price{
				Pair:      pair,
				Amount:    float64(1000 * (i + 1)),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}

		// All should exist initially
		prices, missing := adapter.GetMany(ctx, pairs)
		assert.Len(t, prices, 3)
		assert.Len(t, missing, 0)

		// Wait for expiration
		time.Sleep(50 * time.Millisecond)

		// All should be expired/missing
		prices, missing = adapter.GetMany(ctx, pairs)
		assert.Len(t, prices, 0)
		assert.Len(t, missing, 3)
	})
}

func TestIntegration_MemoryManagement(t *testing.T) {
	memoryCache := NewMemoryCache().(*MemoryCache)
	adapter := NewPriceCache(memoryCache, 20*time.Millisecond)
	ctx := context.Background()

	t.Run("automatic cleanup on set", func(t *testing.T) {
		// Add items that will expire quickly
		for i := 0; i < 10; i++ {
			price := &entities.Price{
				Pair:      fmt.Sprintf("EXPIRE%d/USD", i),
				Amount:    float64(1000 + i),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}

		initialSize := memoryCache.Size()
		assert.Equal(t, 10, initialSize)

		// Wait for expiration
		time.Sleep(30 * time.Millisecond)

		// Add new item, should trigger cleanup
		newPrice := &entities.Price{
			Pair:      "NEW/USD",
			Amount:    5000.0,
			Timestamp: time.Now(),
			Age:       time.Minute,
		}
		err := adapter.Set(ctx, newPrice)
		require.NoError(t, err)

		// Size should be reduced (expired items cleaned up)
		finalSize := memoryCache.Size()
		assert.Less(t, finalSize, initialSize)
		assert.GreaterOrEqual(t, finalSize, 1) // At least the new item
	})

	t.Run("manual cleanup", func(t *testing.T) {
		// Add items with short expiration
		for i := 0; i < 5; i++ {
			price := &entities.Price{
				Pair:      fmt.Sprintf("MANUAL%d/USD", i),
				Amount:    float64(2000 + i),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}

		initialSize := memoryCache.Size()
		assert.GreaterOrEqual(t, initialSize, 5)

		// Wait for expiration
		time.Sleep(30 * time.Millisecond)

		// Manual cleanup
		memoryCache.Cleanup()

		finalSize := memoryCache.Size()
		assert.Less(t, finalSize, initialSize)
	})
}

func TestIntegration_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	memoryCache := NewMemoryCache()
	adapter := NewPriceCache(memoryCache, 10*time.Minute)
	ctx := context.Background()

	t.Run("sequential operations performance", func(t *testing.T) {
		numOperations := 1000

		// Measure Set operations
		start := time.Now()
		for i := 0; i < numOperations; i++ {
			price := &entities.Price{
				Pair:      fmt.Sprintf("PERF%d/USD", i),
				Amount:    float64(i),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}
		setDuration := time.Since(start)

		// Measure Get operations
		start = time.Now()
		for i := 0; i < numOperations; i++ {
			pair := fmt.Sprintf("PERF%d/USD", i)
			price, found := adapter.Get(ctx, pair)
			assert.True(t, found)
			assert.NotNil(t, price)
		}
		getDuration := time.Since(start)

		t.Logf("Set %d items in %v (%.2f ops/sec)",
			numOperations, setDuration, float64(numOperations)/setDuration.Seconds())
		t.Logf("Get %d items in %v (%.2f ops/sec)",
			numOperations, getDuration, float64(numOperations)/getDuration.Seconds())

		// Basic performance assertions (these are very lenient)
		assert.Less(t, setDuration, 1*time.Second, "Set operations should complete within 1 second")
		assert.Less(t, getDuration, 1*time.Second, "Get operations should complete within 1 second")
	})

	t.Run("GetMany performance", func(t *testing.T) {
		// Setup data
		numItems := 100
		pairs := make([]string, numItems)
		for i := 0; i < numItems; i++ {
			pair := fmt.Sprintf("GETMANY%d/USD", i)
			pairs[i] = pair

			price := &entities.Price{
				Pair:      pair,
				Amount:    float64(i * 100),
				Timestamp: time.Now(),
				Age:       time.Minute,
			}
			err := adapter.Set(ctx, price)
			require.NoError(t, err)
		}

		// Measure GetMany
		start := time.Now()
		prices, missing := adapter.GetMany(ctx, pairs)
		duration := time.Since(start)

		assert.Len(t, prices, numItems)
		assert.Len(t, missing, 0)

		t.Logf("GetMany %d items in %v", numItems, duration)
		assert.Less(t, duration, 100*time.Millisecond, "GetMany should be fast")
	})
}
