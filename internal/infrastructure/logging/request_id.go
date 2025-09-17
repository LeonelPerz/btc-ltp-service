package logging

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// RequestIDGenerator generates unique request IDs
type RequestIDGenerator struct {
	prefix string
}

// NewRequestIDGenerator creates a new request ID generator
func NewRequestIDGenerator(prefix string) *RequestIDGenerator {
	if prefix == "" {
		prefix = "req"
	}
	return &RequestIDGenerator{
		prefix: prefix,
	}
}

// Generate creates a new unique request ID
// Format: {prefix}_{timestamp}_{random}
func (g *RequestIDGenerator) Generate() string {
	// Get current timestamp in microseconds for uniqueness
	timestamp := time.Now().UnixMicro()

	// Generate random bytes
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%s_%d", g.prefix, timestamp)
	}

	randomHex := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("%s_%d_%s", g.prefix, timestamp, randomHex)
}

// GenerateShort creates a shorter request ID (for space-constrained contexts)
func (g *RequestIDGenerator) GenerateShort() string {
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%s_%d", g.prefix, time.Now().Unix()%100000)
	}

	return fmt.Sprintf("%s_%s", g.prefix, hex.EncodeToString(randomBytes))
}

// Default request ID generator
var defaultGenerator = NewRequestIDGenerator("req")

// GenerateRequestID generates a request ID using the default generator
func GenerateRequestID() string {
	return defaultGenerator.Generate()
}

// GenerateShortRequestID generates a short request ID using the default generator
func GenerateShortRequestID() string {
	return defaultGenerator.GenerateShort()
}
