package model

import (
	"fmt"
	"time"
)

// LTPResponse represents the Last Traded Price response structure
type LTPResponse struct {
	LTP []LTPPair `json:"ltp"`
}

// LTPPair represents a trading pair with its last traded price
type LTPPair struct {
	Pair   string  `json:"pair"`
	Amount float64 `json:"amount"`
}

// KrakenResponse represents the response from Kraken API
type KrakenResponse struct {
	Error  []string                    `json:"error"`
	Result map[string]KrakenTickerData `json:"result"`
}

// KrakenTickerData represents ticker data from Kraken API
type KrakenTickerData struct {
	LastTradeClosed []string `json:"c"` // [price, lot_volume]
}

// CachedPrice represents a cached price with timestamp
type CachedPrice struct {
	Price     float64
	Timestamp time.Time
}

// PairMappings maps standard pair names to Kraken pair names
// This is a comprehensive list of available pairs that can be configured via environment variables
var PairMappings = map[string]string{
	// Bitcoin pairs
	"BTC/USD": "XXBTZUSD",
	"BTC/CHF": "XBTCHF",
	"BTC/EUR": "XXBTZEUR",
	"BTC/GBP": "XXBTZGBP",
	"BTC/CAD": "XXBTZCAD",
	"BTC/AUD": "XXBTZAUD",
	"BTC/JPY": "XXBTZJPY",

	// Ethereum pairs
	"ETH/USD": "XETHZUSD",
	"ETH/EUR": "XETHZEUR",
	"ETH/CHF": "ETHCHF",
	"ETH/GBP": "XETHZGBP",
	"ETH/CAD": "XETHZCAD",
	"ETH/AUD": "XETHZAUD",
	"ETH/JPY": "XETHZJPY",
	"ETH/BTC": "XETHXXBT",

	// Litecoin pairs
	"LTC/USD": "XLTCZUSD",
	"LTC/EUR": "XLTCZEUR",
	"LTC/BTC": "XLTCXXBT",
}

// SupportedPairs will be populated dynamically based on configuration
// This maintains backward compatibility
var SupportedPairs map[string]string

// KrakenToStandardPair converts Kraken pair names to standard format
var KrakenToStandardPair = map[string]string{
	// Bitcoin pairs
	"XXBTZUSD": "BTC/USD",
	"XXBTZCHF": "BTC/CHF",
	"XXBTZEUR": "BTC/EUR",
	"XXBTZGBP": "BTC/GBP",
	"XXBTZCAD": "BTC/CAD",
	"XXBTZAUD": "BTC/AUD",
	"XXBTZJPY": "BTC/JPY",

	// Ethereum pairs
	"XETHZUSD": "ETH/USD",
	"XETHZEUR": "ETH/EUR",
	"XETHZCHF": "ETH/CHF",
	"XETHZGBP": "ETH/GBP",
	"XETHZCAD": "ETH/CAD",
	"XETHZAUD": "ETH/AUD",
	"XETHZJPY": "ETH/JPY",
	"XETHXXBT": "ETH/BTC",

	// Litecoin pairs
	"XLTCZUSD": "LTC/USD",
	"XLTCZEUR": "LTC/EUR",
	"XLTCXXBT": "LTC/BTC",
}

// InitializeSupportedPairs initializes the SupportedPairs map based on configuration
// This allows dynamic configuration of supported pairs via environment variables
func InitializeSupportedPairs(configuredPairs []string) error {
	if SupportedPairs == nil {
		SupportedPairs = make(map[string]string)
	}

	for _, pair := range configuredPairs {
		if krakenPair, exists := PairMappings[pair]; exists {
			SupportedPairs[pair] = krakenPair
		} else {
			return fmt.Errorf("pair mapping not found for: %s. Available pairs: check PairMappings", pair)
		}
	}

	return nil
}

// GetAvailablePairs returns all available pairs that can be configured
func GetAvailablePairs() []string {
	pairs := make([]string, 0, len(PairMappings))
	for pair := range PairMappings {
		pairs = append(pairs, pair)
	}
	return pairs
}
