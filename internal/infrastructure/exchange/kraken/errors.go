package kraken

import "errors"

var (
	ErrInvalidTickerData = errors.New("invalid ticker data")
	ErrAPIRequest        = errors.New("kraken API request failed")
	ErrInvalidPair       = errors.New("invalid trading pair")
	ErrConnectionFailed  = errors.New("connection to kraken failed")
	ErrWebSocketClosed   = errors.New("websocket connection closed")
	ErrRetryableRequest  = errors.New("retryable kraken API request failed")
	ErrNonRetryable      = errors.New("non-retryable kraken API error")
)
