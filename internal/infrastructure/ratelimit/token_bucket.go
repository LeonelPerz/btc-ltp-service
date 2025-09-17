package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a simple token bucket rate limiter
type TokenBucket struct {
	mu         sync.Mutex
	capacity   int       // Maximum number of tokens
	tokens     int       // Current number of tokens
	refillRate int       // Tokens per second
	lastRefill time.Time // Last refill time
}

// NewTokenBucket creates a new token bucket rate limiter
// capacity: maximum number of tokens in the bucket
// refillRate: number of tokens added per second
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // Start with full bucket
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if available
// Returns true if request is allowed, false if rate limited
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	tb.refill()

	// Check if we have tokens available
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// AllowN checks if N tokens are available and consumes them if so
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	tb.refill()

	// Check if we have enough tokens
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}

	return false
}

// Tokens returns the current number of available tokens
func (tb *TokenBucket) Tokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return tb.tokens
}

// refill adds tokens based on elapsed time since last refill
// Must be called with lock held
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Calculate tokens to add based on refill rate
	tokensToAdd := int(elapsed.Seconds() * float64(tb.refillRate))

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd

		// Cap at bucket capacity
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}

		tb.lastRefill = now
	}
}

// RateLimiterCollection manages multiple token buckets for different clients
type RateLimiterCollection struct {
	mu         sync.RWMutex
	buckets    map[string]*TokenBucket
	capacity   int
	refillRate int
	// Cleanup old buckets to prevent memory leak
	lastCleanup     time.Time
	cleanupInterval time.Duration
}

// NewRateLimiterCollection creates a new collection of rate limiters
func NewRateLimiterCollection(capacity, refillRate int) *RateLimiterCollection {
	return &RateLimiterCollection{
		buckets:         make(map[string]*TokenBucket),
		capacity:        capacity,
		refillRate:      refillRate,
		lastCleanup:     time.Now(),
		cleanupInterval: 10 * time.Minute, // Clean up every 10 minutes
	}
}

// Allow checks if a request from the given client is allowed
func (rlc *RateLimiterCollection) Allow(clientID string) bool {
	bucket := rlc.getBucket(clientID)
	return bucket.Allow()
}

// AllowN checks if N requests from the given client are allowed
func (rlc *RateLimiterCollection) AllowN(clientID string, n int) bool {
	bucket := rlc.getBucket(clientID)
	return bucket.AllowN(n)
}

// Tokens returns available tokens for the given client
func (rlc *RateLimiterCollection) Tokens(clientID string) int {
	bucket := rlc.getBucket(clientID)
	return bucket.Tokens()
}

// getBucket gets or creates a token bucket for the client
func (rlc *RateLimiterCollection) getBucket(clientID string) *TokenBucket {
	// Try read lock first for better performance
	rlc.mu.RLock()
	bucket, exists := rlc.buckets[clientID]
	rlc.mu.RUnlock()

	if exists {
		return bucket
	}

	// Need to create bucket, acquire write lock
	rlc.mu.Lock()
	defer rlc.mu.Unlock()

	// Double-check pattern - another goroutine might have created it
	if bucket, exists := rlc.buckets[clientID]; exists {
		return bucket
	}

	// Create new bucket
	bucket = NewTokenBucket(rlc.capacity, rlc.refillRate)
	rlc.buckets[clientID] = bucket

	// Opportunistic cleanup to prevent memory leaks
	rlc.maybeCleanup()

	return bucket
}

// maybeCleanup removes old unused buckets to prevent memory leaks
// Must be called with write lock held
func (rlc *RateLimiterCollection) maybeCleanup() {
	now := time.Now()
	if now.Sub(rlc.lastCleanup) < rlc.cleanupInterval {
		return
	}

	// Remove buckets that haven't been used recently and are full
	// (indicating they haven't been active)
	cutoff := now.Add(-30 * time.Minute) // Remove buckets unused for 30 minutes

	for clientID, bucket := range rlc.buckets {
		// If bucket is full and hasn't been refilled recently, it's likely unused
		if bucket.tokens == bucket.capacity && bucket.lastRefill.Before(cutoff) {
			delete(rlc.buckets, clientID)
		}
	}

	rlc.lastCleanup = now
}

// Stats returns statistics about the rate limiter collection
func (rlc *RateLimiterCollection) Stats() map[string]interface{} {
	rlc.mu.RLock()
	defer rlc.mu.RUnlock()

	return map[string]interface{}{
		"total_clients": len(rlc.buckets),
		"capacity":      rlc.capacity,
		"refill_rate":   rlc.refillRate,
	}
}
