package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryCache_CacheEviction_Advanced tests advanced cache eviction scenarios
func TestMemoryCache_CacheEviction_Advanced(t *testing.T) {
	tests := []struct {
		name            string
		setupItems      map[string]time.Duration
		waitTime        time.Duration
		expectedAlive   []string
		expectedEvicted []string
	}{
		{
			name: "eviction_based_on_ttl_order",
			setupItems: map[string]time.Duration{
				"short":  10 * time.Millisecond,
				"medium": 50 * time.Millisecond,
				"long":   200 * time.Millisecond,
			},
			waitTime:        30 * time.Millisecond,
			expectedAlive:   []string{"medium", "long"},
			expectedEvicted: []string{"short"},
		},
		{
			name: "partial_eviction",
			setupItems: map[string]time.Duration{
				"expired1": 5 * time.Millisecond,
				"expired2": 10 * time.Millisecond,
				"valid1":   1 * time.Hour,
				"valid2":   2 * time.Hour,
			},
			waitTime:        20 * time.Millisecond,
			expectedAlive:   []string{"valid1", "valid2"},
			expectedEvicted: []string{"expired1", "expired2"},
		},
		{
			name: "no_eviction_all_valid",
			setupItems: map[string]time.Duration{
				"item1": 1 * time.Hour,
				"item2": 2 * time.Hour,
				"item3": 3 * time.Hour,
			},
			waitTime:        10 * time.Millisecond,
			expectedAlive:   []string{"item1", "item2", "item3"},
			expectedEvicted: []string{},
		},
		{
			name: "complete_eviction",
			setupItems: map[string]time.Duration{
				"temp1": 5 * time.Millisecond,
				"temp2": 8 * time.Millisecond,
				"temp3": 12 * time.Millisecond,
			},
			waitTime:        20 * time.Millisecond,
			expectedAlive:   []string{},
			expectedEvicted: []string{"temp1", "temp2", "temp3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMemoryCache().(*MemoryCache)
			ctx := context.Background()

			// Setup items with different TTLs
			for key, ttl := range tt.setupItems {
				err := cache.Set(ctx, key, fmt.Sprintf("value-%s", key), ttl)
				require.NoError(t, err)
			}

			// Verify all items are initially present
			assert.Equal(t, len(tt.setupItems), cache.Size())

			// Wait for some items to expire
			time.Sleep(tt.waitTime)

			// Check alive items
			for _, key := range tt.expectedAlive {
				value, err := cache.Get(ctx, key)
				assert.NoError(t, err, "Expected key '%s' to be alive", key)
				assert.Equal(t, fmt.Sprintf("value-%s", key), value)
			}

			// Check evicted items
			for _, key := range tt.expectedEvicted {
				value, err := cache.Get(ctx, key)
				assert.Equal(t, ErrKeyExpired, err, "Expected key '%s' to be evicted", key)
				assert.Equal(t, "", value)
			}
		})
	}
}

// TestMemoryCache_TTL_EdgeCases tests TTL behavior in edge cases
func TestMemoryCache_TTL_EdgeCases(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	t.Run("zero_ttl_immediate_expiry", func(t *testing.T) {
		err := cache.Set(ctx, "zero-ttl", "value", 0)
		require.NoError(t, err)

		// Should expire immediately
		value, err := cache.Get(ctx, "zero-ttl")
		assert.Equal(t, ErrKeyExpired, err)
		assert.Equal(t, "", value)
	})

	t.Run("negative_ttl_pre_expired", func(t *testing.T) {
		err := cache.Set(ctx, "negative-ttl", "value", -1*time.Hour)
		require.NoError(t, err)

		// Should be expired from the start
		value, err := cache.Get(ctx, "negative-ttl")
		assert.Equal(t, ErrKeyExpired, err)
		assert.Equal(t, "", value)
	})

	t.Run("microsecond_ttl", func(t *testing.T) {
		err := cache.Set(ctx, "micro-ttl", "value", 100*time.Microsecond)
		require.NoError(t, err)

		// Immediately try to get it - might work if CPU is fast enough
		value1, err1 := cache.Get(ctx, "micro-ttl")

		// Wait a bit and try again - should definitely be expired
		time.Sleep(200 * time.Microsecond)
		value2, err2 := cache.Get(ctx, "micro-ttl")

		// At least the second attempt should show expiration
		if err1 == nil {
			assert.Equal(t, "value", value1)
		}
		assert.Equal(t, ErrKeyExpired, err2)
		assert.Equal(t, "", value2)
	})

	t.Run("very_long_ttl", func(t *testing.T) {
		longTTL := 365 * 24 * time.Hour // 1 year
		err := cache.Set(ctx, "long-ttl", "persistent-value", longTTL)
		require.NoError(t, err)

		value, err := cache.Get(ctx, "long-ttl")
		assert.NoError(t, err)
		assert.Equal(t, "persistent-value", value)
	})
}

// TestMemoryCache_AutoEviction_OnSet tests automatic eviction during Set operations
func TestMemoryCache_AutoEviction_OnSet(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	// Add items that will expire soon
	_ = cache.Set(ctx, "expired1", "value1", 1*time.Nanosecond)
	_ = cache.Set(ctx, "expired2", "value2", 1*time.Nanosecond)
	_ = cache.Set(ctx, "expired3", "value3", 1*time.Nanosecond)
	_ = cache.Set(ctx, "valid", "valid-value", 1*time.Hour)

	// Wait for expiration
	time.Sleep(2 * time.Nanosecond)

	initialSize := cache.Size()
	t.Logf("Size before cleanup: %d", initialSize)

	// Set a new item - should trigger auto-cleanup
	err := cache.Set(ctx, "new-item", "new-value", 1*time.Hour)
	require.NoError(t, err)

	finalSize := cache.Size()
	t.Logf("Size after cleanup: %d", finalSize)

	// Should have cleaned up expired items, keeping only 'valid' and 'new-item'
	assert.Equal(t, 2, finalSize)

	// Verify that valid items are still accessible
	value, err := cache.Get(ctx, "valid")
	assert.NoError(t, err)
	assert.Equal(t, "valid-value", value)

	value, err = cache.Get(ctx, "new-item")
	assert.NoError(t, err)
	assert.Equal(t, "new-value", value)
}

// TestMemoryCache_ConcurrentEviction tests eviction under concurrent access
func TestMemoryCache_ConcurrentEviction(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	numGoroutines := 50
	numItemsPerGoroutine := 10

	var wg sync.WaitGroup

	// Concurrent writers with varying TTLs
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numItemsPerGoroutine; j++ {
				key := fmt.Sprintf("worker-%d-item-%d", goroutineID, j)

				// Vary TTL: some short, some long
				var ttl time.Duration
				if j%3 == 0 {
					ttl = 10 * time.Millisecond // Short TTL
				} else {
					ttl = 1 * time.Hour // Long TTL
				}

				value := fmt.Sprintf("value-%d-%d", goroutineID, j)
				_ = cache.Set(ctx, key, value, ttl)
			}
		}(i)
	}

	// Concurrent readers trying to access items
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numItemsPerGoroutine; j++ {
				key := fmt.Sprintf("worker-%d-item-%d", goroutineID, j)
				_, _ = cache.Get(ctx, key) // Ignore results to avoid races
			}
		}(i)
	}

	wg.Wait()

	// Wait for short TTL items to expire
	time.Sleep(50 * time.Millisecond)

	// Force cleanup
	cache.Cleanup()

	// Verify that some items have been evicted
	finalSize := cache.Size()
	totalItems := numGoroutines * numItemsPerGoroutine
	expectedEvicted := totalItems / 3 // Items with short TTL
	expectedRemaining := totalItems - expectedEvicted

	t.Logf("Total items added: %d", totalItems)
	t.Logf("Expected evicted: %d", expectedEvicted)
	t.Logf("Expected remaining: %d", expectedRemaining)
	t.Logf("Actual final size: %d", finalSize)

	// Should have evicted the short TTL items
	assert.LessOrEqual(t, finalSize, expectedRemaining)
}

// TestMemoryCache_EvictionCleanupTiming tests the timing of cleanup operations
func TestMemoryCache_EvictionCleanupTiming(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	// Add many items with short TTL
	numItems := 100
	for i := 0; i < numItems; i++ {
		key := fmt.Sprintf("item-%d", i)
		_ = cache.Set(ctx, key, fmt.Sprintf("value-%d", i), 5*time.Millisecond)
	}

	assert.Equal(t, numItems, cache.Size())

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Items are expired but still in memory until cleanup
	assert.Equal(t, numItems, cache.Size())

	// Manual cleanup should remove all expired items
	start := time.Now()
	cache.Cleanup()
	cleanupDuration := time.Since(start)

	t.Logf("Cleanup of %d expired items took: %v", numItems, cleanupDuration)

	// All items should be removed after cleanup
	assert.Equal(t, 0, cache.Size())

	// Cleanup should be reasonably fast
	assert.Less(t, cleanupDuration, 100*time.Millisecond, "Cleanup took too long")
}

// TestMemoryCache_MemoryEvictionUnderPressure tests behavior under memory pressure
func TestMemoryCache_MemoryEvictionUnderPressure(t *testing.T) {
	cache := NewMemoryCache().(*MemoryCache)
	ctx := context.Background()

	// Add many items that expire at different times
	batchSize := 50

	// Batch 1: Expire in 10ms
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("short-%d", i)
		_ = cache.Set(ctx, key, "short-lived", 10*time.Millisecond)
	}

	// Batch 2: Expire in 30ms
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("medium-%d", i)
		_ = cache.Set(ctx, key, "medium-lived", 30*time.Millisecond)
	}

	// Batch 3: Long-lived
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("long-%d", i)
		_ = cache.Set(ctx, key, "long-lived", 1*time.Hour)
	}

	totalItems := 3 * batchSize
	assert.Equal(t, totalItems, cache.Size())

	// Wait for first batch to expire
	time.Sleep(15 * time.Millisecond)

	// Adding more items should trigger auto-cleanup of first batch
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("new-%d", i)
		_ = cache.Set(ctx, key, "new-value", 1*time.Hour)
	}

	// Should have fewer items now (first batch cleaned up)
	currentSize := cache.Size()
	t.Logf("Size after first batch expiry: %d (expected <= %d)", currentSize, totalItems-batchSize+10)

	// Wait for second batch to expire
	time.Sleep(25 * time.Millisecond)

	// Another cleanup trigger
	_ = cache.Set(ctx, "trigger-cleanup", "cleanup-trigger", 1*time.Hour)

	finalSize := cache.Size()
	expectedFinalSize := batchSize + 10 + 1 // long-lived + new + trigger

	t.Logf("Final size: %d (expected ~%d)", finalSize, expectedFinalSize)
	assert.LessOrEqual(t, finalSize, expectedFinalSize+5) // Allow some variance for timing
}
