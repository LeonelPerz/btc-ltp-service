package model

import "time"

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
	Error  []string                      `json:"error"`
	Result map[string]KrakenTickerData   `json:"result"`
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

// SupportedPairs maps our pair names to Kraken pair names
var SupportedPairs = map[string]string{
	"BTC/USD": "XBTUSD",
	"BTC/CHF": "XBTCHF", 
	"BTC/EUR": "XBTEUR",
}

// KrakenToStandardPair converts Kraken pair names to standard format
var KrakenToStandardPair = map[string]string{
	"XXBTZUSD": "BTC/USD",
	"XBTCHF":   "BTC/CHF",
	"XXBTZEUR": "BTC/EUR",
}
