package kraken

import (
	"strconv"
	"time"
)

// KrakenTickerResponse representa la respuesta de la API de Kraken para tickers
type KrakenTickerResponse struct {
	Error  []string                    `json:"error"`
	Result map[string]KrakenTickerData `json:"result"`
}

// KrakenTickerData representa los datos del ticker para un par específico
// Usando interface{} para campos que pueden tener diferentes tipos
type KrakenTickerData struct {
	Ask                 []string      `json:"a"` // ask array(<price>, <whole lot volume>, <lot volume>),
	Bid                 []string      `json:"b"` // bid array(<price>, <whole lot volume>, <lot volume>),
	LastTradeClosed     []string      `json:"c"` // last trade closed array(<price>, <lot volume>),
	Volume              []string      `json:"v"` // volume array(<today>, <last 24 hours>),
	VolumeWeightedPrice []string      `json:"p"` // volume weighted average price array(<today>, <last 24 hours>),
	NumberOfTrades      []interface{} `json:"t"` // number of trades array(<today>, <last 24 hours>),
	Low                 []string      `json:"l"` // low array(<today>, <last 24 hours>),
	High                []string      `json:"h"` // high array(<today>, <last 24 hours>),
	OpeningPrice        interface{}   `json:"o"` // today's opening price (can be string or array)
}

// GetLastTradedPrice extrae el último precio de trading del ticker
func (t *KrakenTickerData) GetLastTradedPrice() (float64, error) {
	if len(t.LastTradeClosed) == 0 {
		return 0, ErrInvalidTickerData
	}
	return strconv.ParseFloat(t.LastTradeClosed[0], 64)
}

// GetTimestamp retorna el timestamp actual ya que Kraken no proporciona timestamp en el ticker
func (t *KrakenTickerData) GetTimestamp() time.Time {
	return time.Now()
}

// GetAge calcula la edad del precio (siempre 0 para datos en tiempo real)
func (t *KrakenTickerData) GetAge() time.Duration {
	return 0
}
