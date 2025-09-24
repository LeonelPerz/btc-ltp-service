package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTradingPairValidation_Comprehensive tests comprehensive trading pair validation
func TestTradingPairValidation_Comprehensive(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		pairs         []string
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:        "valid_major_pairs",
			pairs:       []string{"BTC/USD", "ETH/USD", "BTC/EUR"},
			expectError: false,
			description: "Major trading pairs should be valid",
		},
		{
			name:        "valid_case_insensitive",
			pairs:       []string{"btc/usd", "Eth/Eur", "XRP/USD"},
			expectError: false,
			description: "Case insensitive pairs should be normalized and validated",
		},
		{
			name:        "valid_altcoins",
			pairs:       []string{"ADA/USD", "DOT/EUR", "LINK/USD", "UNI/EUR"},
			expectError: false,
			description: "Popular altcoin pairs should be valid",
		},
		{
			name:          "invalid_empty_list",
			pairs:         []string{},
			expectError:   true,
			errorContains: "cannot be empty",
			description:   "Empty pair list should fail validation",
		},
		{
			name:          "invalid_nil_list",
			pairs:         nil,
			expectError:   true,
			errorContains: "cannot be empty",
			description:   "Nil pair list should fail validation",
		},
		{
			name:          "invalid_format_no_slash",
			pairs:         []string{"BTCUSD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pairs without slash should fail",
		},
		{
			name:          "invalid_format_multiple_slashes",
			pairs:         []string{"BTC/USD/EUR", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pairs with multiple slashes should fail",
		},
		{
			name:          "invalid_format_double_slash",
			pairs:         []string{"BTC//USD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pairs with double slash should fail",
		},
		{
			name:          "invalid_empty_base",
			pairs:         []string{"/USD", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pairs with empty base currency should fail",
		},
		{
			name:          "invalid_empty_quote",
			pairs:         []string{"BTC/", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pairs with empty quote currency should fail",
		},
		{
			name:          "invalid_unknown_pair",
			pairs:         []string{"ABC/XYZ", "ETH/USD"},
			expectError:   true,
			errorContains: "unknown trading pairs",
			description:   "Unknown trading pairs should fail",
		},
		{
			name:          "invalid_unknown_base_currency",
			pairs:         []string{"FAKECOIN/USD", "ETH/USD"},
			expectError:   true,
			errorContains: "unknown trading pairs",
			description:   "Pairs with unknown base currency should fail",
		},
		{
			name:          "invalid_unknown_quote_currency",
			pairs:         []string{"BTC/FAKEUNIT", "ETH/USD"},
			expectError:   true,
			errorContains: "unknown trading pairs",
			description:   "Pairs with unknown quote currency should fail",
		},
		{
			name:          "invalid_only_slash",
			pairs:         []string{"/", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Pair consisting only of slash should fail",
		},
		{
			name:          "invalid_whitespace_only",
			pairs:         []string{"   ", "ETH/USD"},
			expectError:   true,
			errorContains: "invalid pair format",
			description:   "Whitespace-only pairs should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateTradingPairs(tt.pairs)

			if tt.expectError {
				require.Error(t, err, tt.description)
				if tt.errorContains != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorContains),
						"Error message should contain expected text")
				}
				t.Logf("Expected error occurred: %v", err)
			} else {
				assert.NoError(t, err, tt.description)
				t.Logf("Validation passed for pairs: %v", tt.pairs)
			}
		})
	}
}

// TestTradingPairValidation_KnownPairs tests validation against known Kraken pairs
func TestTradingPairValidation_KnownPairs(t *testing.T) {
	validator := NewValidator()
	knownPairs := validator.getKnownKrakenPairs()

	t.Run("all_known_pairs_valid", func(t *testing.T) {
		// Extract all known pairs
		var allKnownPairs []string
		for pair := range knownPairs {
			allKnownPairs = append(allKnownPairs, pair)
		}

		// All should pass validation
		err := validator.validateTradingPairs(allKnownPairs)
		assert.NoError(t, err)
		t.Logf("Validated %d known pairs successfully", len(allKnownPairs))
	})

	t.Run("known_pairs_content", func(t *testing.T) {
		// Verify some expected pairs are present
		expectedPairs := []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD"}

		for _, pair := range expectedPairs {
			assert.True(t, knownPairs[pair], "Expected pair %s should be in known pairs", pair)
		}

		// Verify minimum number of pairs
		assert.GreaterOrEqual(t, len(knownPairs), 15, "Should have at least 15 known pairs")
	})

	t.Run("case_insensitive_known_pairs", func(t *testing.T) {
		// Test that case variations of known pairs work
		casePairs := []string{"btc/usd", "BTC/USD", "Btc/Usd", "ETH/eur", "eth/EUR"}

		err := validator.validateTradingPairs(casePairs)
		assert.NoError(t, err)
	})
}

// TestTradingPairValidation_ErrorMessages tests that error messages are informative
func TestTradingPairValidation_ErrorMessages(t *testing.T) {
	validator := NewValidator()

	t.Run("format_error_details", func(t *testing.T) {
		pairs := []string{"BTCUSD", "INVALID", "ETH-USD"}
		err := validator.validateTradingPairs(pairs)

		require.Error(t, err)
		errorMsg := err.Error()

		// Should contain specific format guidance
		assert.Contains(t, errorMsg, "BASE/QUOTE")
		assert.Contains(t, errorMsg, "invalid pair format")
		t.Logf("Format error message: %s", errorMsg)
	})

	t.Run("unknown_pairs_error_details", func(t *testing.T) {
		pairs := []string{"FAKE/COIN", "INVALID/PAIR", "BTC/USD"}
		err := validator.validateTradingPairs(pairs)

		require.Error(t, err)
		errorMsg := err.Error()

		// Should list the unknown pairs and show examples of valid ones
		assert.Contains(t, errorMsg, "unknown trading pairs")
		assert.Contains(t, errorMsg, "FAKE/COIN")
		assert.Contains(t, errorMsg, "INVALID/PAIR")
		assert.Contains(t, errorMsg, "supported pairs")
		t.Logf("Unknown pairs error message: %s", errorMsg)
	})
}

// TestTradingPairValidation_EdgeCases tests edge cases in pair validation
func TestTradingPairValidation_EdgeCases(t *testing.T) {
	validator := NewValidator()

	t.Run("whitespace_handling", func(t *testing.T) {
		// Pairs with various whitespace should be handled
		pairs := []string{" BTC/USD ", "ETH/USD", "\tLTC/USD\t", "\nXRP/USD\n"}

		err := validator.validateTradingPairs(pairs)
		assert.NoError(t, err)
	})

	t.Run("duplicate_pairs", func(t *testing.T) {
		// Duplicate pairs should be allowed (de-duplication is not the validator's job)
		pairs := []string{"BTC/USD", "ETH/USD", "BTC/USD", "LTC/USD"}

		err := validator.validateTradingPairs(pairs)
		assert.NoError(t, err)
	})

	t.Run("single_pair", func(t *testing.T) {
		// Single pair should work
		pairs := []string{"BTC/USD"}

		err := validator.validateTradingPairs(pairs)
		assert.NoError(t, err)
	})

	t.Run("large_pair_list", func(t *testing.T) {
		// Large list of valid pairs should work efficiently
		var largePairList []string
		knownPairs := validator.getKnownKrakenPairs()

		// Repeat known pairs multiple times
		for i := 0; i < 10; i++ {
			for pair := range knownPairs {
				largePairList = append(largePairList, pair)
			}
		}

		err := validator.validateTradingPairs(largePairList)
		assert.NoError(t, err)
		t.Logf("Successfully validated %d pairs", len(largePairList))
	})
}

// TestIsKnownPair tests the isKnownPair helper function
func TestIsKnownPair(t *testing.T) {
	validator := NewValidator()
	knownPairs := validator.getKnownKrakenPairs()

	tests := []struct {
		name     string
		pair     string
		expected bool
	}{
		{"known_pair_exact", "BTC/USD", true},
		{"known_pair_case_insensitive", "btc/usd", true},
		{"unknown_pair", "FAKE/COIN", false},
		{"empty_pair", "", false},
		{"invalid_format", "BTCUSD", false},
		{"partial_match_base", "BTC/UNKNOWN", false},
		{"partial_match_quote", "UNKNOWN/USD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.isKnownPair(tt.pair, knownPairs)
			assert.Equal(t, tt.expected, result,
				"isKnownPair(%q) expected %v, got %v", tt.pair, tt.expected, result)
		})
	}
}

// TestGetSampleKnownPairs tests the sample pairs function used in error messages
func TestGetSampleKnownPairs(t *testing.T) {
	validator := NewValidator()
	samples := validator.getSampleKnownPairs()

	assert.NotEmpty(t, samples, "Sample pairs should not be empty")
	assert.LessOrEqual(t, len(samples), 10, "Should provide a reasonable number of samples")

	// All samples should be valid pairs
	for _, pair := range samples {
		assert.Contains(t, pair, "/", "Sample pair should contain slash: %s", pair)
		parts := strings.Split(pair, "/")
		assert.Len(t, parts, 2, "Sample pair should have exactly one slash: %s", pair)
		assert.NotEmpty(t, parts[0], "Base currency should not be empty: %s", pair)
		assert.NotEmpty(t, parts[1], "Quote currency should not be empty: %s", pair)
	}

	t.Logf("Sample known pairs: %v", samples)
}

// BenchmarkTradingPairValidation benchmarks the validation performance
func BenchmarkTradingPairValidation(b *testing.B) {
	validator := NewValidator()

	// Test with different sized pair lists
	smallPairs := []string{"BTC/USD", "ETH/USD", "LTC/USD"}
	largePairs := make([]string, 0, 100)
	knownPairs := validator.getKnownKrakenPairs()

	// Create a large list by repeating known pairs
	count := 0
	for pair := range knownPairs {
		if count >= 100 {
			break
		}
		largePairs = append(largePairs, pair)
		count++
	}

	b.Run("small_pair_list", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validator.validateTradingPairs(smallPairs)
		}
	})

	b.Run("large_pair_list", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validator.validateTradingPairs(largePairs)
		}
	})

	b.Run("invalid_pairs", func(b *testing.B) {
		invalidPairs := []string{"INVALID1", "FAKE/COIN", "BAD-FORMAT", "UNKNOWN/PAIR"}
		for i := 0; i < b.N; i++ {
			_ = validator.validateTradingPairs(invalidPairs)
		}
	})
}
