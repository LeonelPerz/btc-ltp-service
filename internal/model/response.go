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

// DEPRECATED: Legacy pair mappings - These are kept for backward compatibility only
// Use the new PairMapper service for dynamic pair mapping
var PairMappings = map[string]string{
	// Bitcoin pairs
	"BTC/USD": "XXBTZUSD",
	"BTC/EUR": "XXBTZEUR",
	"BTC/GBP": "XXBTZGBP",
	"BTC/CAD": "XXBTZCAD",
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
// This maintains backward compatibility but is DEPRECATED
var SupportedPairs map[string]string

// DEPRECATED: KrakenToStandardPair - Use PairMapper service instead
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
	"XETHZGBP": "ETH/GBP",
	"XETHZCAD": "ETH/CAD",
	"XETHZJPY": "ETH/JPY",
	"XETHXXBT": "ETH/BTC",

	// Litecoin pairs
	"XLTCZUSD": "LTC/USD",
	"XLTCZEUR": "LTC/EUR",
	"XLTCXXBT": "LTC/BTC",
}

// DEPRECATED: InitializeSupportedPairs - Use PairMapper service instead
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

// DEPRECATED: GetAvailablePairs - Use PairMapper service instead
func GetAvailablePairs() []string {
	pairs := make([]string, 0, len(PairMappings))
	for pair := range PairMappings {
		pairs = append(pairs, pair)
	}
	return pairs
}

// WebSocket message structures for Kraken WebSocket API

// KrakenWSMessage represents a generic WebSocket message
type KrakenWSMessage struct {
	Event        string      `json:"event,omitempty"`
	Pair         []string    `json:"pair,omitempty"`
	Subscription interface{} `json:"subscription,omitempty"`
	Data         interface{} `json:"data,omitempty"`
	ChannelID    int         `json:"channelID,omitempty"`
	ChannelName  string      `json:"channelName,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
}

// KrakenWSSubscription represents subscription details
type KrakenWSSubscription struct {
	Name string `json:"name"`
}

// KrakenWSTickerData represents ticker data from WebSocket
type KrakenWSTickerData struct {
	Ask    []string `json:"a,omitempty"` // [price, whole_lot_volume, lot_volume]
	Bid    []string `json:"b,omitempty"` // [price, whole_lot_volume, lot_volume]
	Close  []string `json:"c,omitempty"` // [price, lot_volume] - Last trade closed
	Volume []string `json:"v,omitempty"` // [today, last_24_hours]
	VWAP   []string `json:"p,omitempty"` // [today, last_24_hours] - volume weighted average price
	Trades []int    `json:"t,omitempty"` // [today, last_24_hours] - number of trades
	Low    []string `json:"l,omitempty"` // [today, last_24_hours]
	High   []string `json:"h,omitempty"` // [today, last_24_hours]
	Open   []string `json:"o,omitempty"` // [today, last_24_hours]
}
