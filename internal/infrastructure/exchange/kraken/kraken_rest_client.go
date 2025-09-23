package kraken

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/metrics"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
)

const (
	KrakenAPIBaseURL = "https://api.kraken.com/0/public"
	DefaultTimeout   = 10 * time.Second
	RequestTimeout   = 3 * time.Second // Context timeout per request
	MaxRetries       = 3               // Maximum retry attempts
	BaseBackoff      = 100 * time.Millisecond
	MaxBackoff       = 2 * time.Second
)

// RestClient implementa la interfaz Exchange usando la API REST de Kraken
type RestClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRestClient crea una nueva instancia del cliente REST de Kraken
func NewRestClient() *RestClient {
	return &RestClient{
		baseURL: KrakenAPIBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// NewRestClientWithConfig crea una nueva instancia del cliente REST de Kraken con configuración
func NewRestClientWithConfig(cfg config.KrakenConfig) *RestClient {
	return &RestClient{
		baseURL: cfg.RestURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// GetTicker obtiene el precio de un par específico con context y retry
func (k *RestClient) GetTicker(ctx context.Context, pair string) (*entities.Price, error) {
	krakenPair, err := toKrakenPair(pair)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pair to kraken pair: %w", err)
	}

	var price *entities.Price

	retryErr := retry.Do(
		func() error {
			// Create request context with timeout
			reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
			defer cancel()

			reqPrice, reqErr := k.doTickerRequest(reqCtx, krakenPair, pair)
			if reqErr != nil {
				return reqErr
			}

			price = reqPrice
			return nil
		},
		retry.Attempts(MaxRetries),
		retry.Delay(BaseBackoff),
		retry.MaxDelay(MaxBackoff),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(k.isRetryableError),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			metrics.RecordExternalAPIRetry("kraken", "/Ticker", int(n+1))

			// Record specific metrics for 429 rate limiting
			if strings.Contains(err.Error(), "HTTP 429") {
				metrics.RecordKrakenRateLimitDrop("/Ticker")
				// Calculate backoff duration for this attempt
				backoffDuration := time.Duration(n+1) * BaseBackoff
				if backoffDuration > MaxBackoff {
					backoffDuration = MaxBackoff
				}
				metrics.RecordKrakenBackoffDuration("/Ticker", int(n+1), backoffDuration.Seconds())
			}

			logging.Warn(ctx, "Kraken API retry attempt", logging.Fields{
				"service":      "kraken",
				"operation":    "GetTicker",
				"attempt":      n + 1,
				"max_attempts": MaxRetries,
				"pair":         pair,
				"error":        err.Error(),
				"is_429":       strings.Contains(err.Error(), "HTTP 429"),
			})
		}),
	)

	if retryErr != nil {
		return nil, fmt.Errorf("failed to get ticker after retries for %s: %w", pair, retryErr)
	}

	return price, nil
}

// doTickerRequest performs the actual HTTP request for a single ticker
func (k *RestClient) doTickerRequest(ctx context.Context, krakenPair, originalPair string) (*entities.Price, error) {
	url := fmt.Sprintf("%s/Ticker?pair=%s", k.baseURL, krakenPair)

	logging.Debug(ctx, "Making request to Kraken API", logging.Fields{
		"url":         url,
		"pair":        originalPair,
		"kraken_pair": krakenPair,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	requestStart := time.Now()
	resp, err := k.httpClient.Do(req)
	requestDuration := time.Since(requestStart)

	if err != nil {
		logging.ErrorWithError(ctx, "Kraken API request failed", err, logging.Fields{
			"url":                 url,
			"pair":                originalPair,
			"request_duration_ms": float64(requestDuration.Nanoseconds()) / 1e6,
		})

		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("%w: context timeout/canceled", ErrRetryableRequest)
		}
		return nil, fmt.Errorf("%w: %v", ErrRetryableRequest, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check for retryable HTTP status codes
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (server error)", ErrRetryableRequest, resp.StatusCode)
	}

	// CRITICAL FIX: Handle HTTP 429 (Too Many Requests) as retryable
	if resp.StatusCode == http.StatusTooManyRequests {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (rate limited by kraken)", ErrRetryableRequest, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (client error)", ErrNonRetryable, resp.StatusCode)
	}

	var tickerResp KrakenTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %w", ErrRetryableRequest, err)
	}

	if len(tickerResp.Error) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrNonRetryable, strings.Join(tickerResp.Error, ", "))
	}

	// Kraken puede devolver el par con un formato diferente, tomamos el primero
	for _, tickerData := range tickerResp.Result {
		price, err := tickerData.GetLastTradedPrice()
		if err != nil {
			return nil, fmt.Errorf("failed to get last traded price: %w", err)
		}

		priceEntity := entities.NewPrice(
			originalPair,
			price,
			tickerData.GetTimestamp(),
			tickerData.GetAge(),
		)

		// Record metrics and logging for successful external API call
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		logging.ExternalRequest(ctx, "kraken", url, float64(requestDuration.Nanoseconds())/1e6, resp.StatusCode, logging.Fields{
			"pair":   originalPair,
			"amount": price,
		})

		return priceEntity, nil
	}

	logging.Error(ctx, "No ticker data found in Kraken response", logging.Fields{
		"url":                 url,
		"pair":                originalPair,
		"request_duration_ms": float64(requestDuration.Nanoseconds()) / 1e6,
		"status_code":         resp.StatusCode,
	})
	return nil, fmt.Errorf("%w: no ticker data found for pair %s", ErrNonRetryable, originalPair)
}

// isRetryableError determines if an error should trigger a retry
func (k *RestClient) isRetryableError(err error) bool {
	return errors.Is(err, ErrRetryableRequest) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

// GetTickers obtiene los precios de múltiples pares con context y retry
func (k *RestClient) GetTickers(ctx context.Context, pairs []string) ([]*entities.Price, error) {
	if len(pairs) == 0 {
		return []*entities.Price{}, nil
	}

	krakenPairs := make([]string, len(pairs))
	for i, pair := range pairs {
		krakenPair, err := toKrakenPair(pair)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to convert pair to kraken pair: %w", ErrInvalidPair, err)
		}
		krakenPairs[i] = krakenPair
	}

	var prices []*entities.Price

	retryErr := retry.Do(
		func() error {
			// Create request context with timeout
			reqCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
			defer cancel()

			reqPrices, reqErr := k.doTickersRequest(reqCtx, krakenPairs, pairs)
			if reqErr != nil {
				return reqErr
			}

			prices = reqPrices
			return nil
		},
		retry.Attempts(MaxRetries),
		retry.Delay(BaseBackoff),
		retry.MaxDelay(MaxBackoff),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(k.isRetryableError),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			metrics.RecordExternalAPIRetry("kraken", "/Ticker", int(n+1))

			// Record specific metrics for 429 rate limiting
			if strings.Contains(err.Error(), "HTTP 429") {
				metrics.RecordKrakenRateLimitDrop("/Ticker")
				// Calculate backoff duration for this attempt
				backoffDuration := time.Duration(n+1) * BaseBackoff
				if backoffDuration > MaxBackoff {
					backoffDuration = MaxBackoff
				}
				metrics.RecordKrakenBackoffDuration("/Ticker", int(n+1), backoffDuration.Seconds())
			}

			logging.Warn(ctx, "Kraken API retry attempt", logging.Fields{
				"service":      "kraken",
				"operation":    "GetTickers",
				"attempt":      n + 1,
				"max_attempts": MaxRetries,
				"pairs_count":  len(pairs),
				"error":        err.Error(),
				"is_429":       strings.Contains(err.Error(), "HTTP 429"),
			})
		}),
	)

	if retryErr != nil {
		return nil, fmt.Errorf("failed to get tickers after retries for %d pairs: %w", len(pairs), retryErr)
	}

	return prices, nil
}

// doTickersRequest performs the actual HTTP request for multiple tickers
func (k *RestClient) doTickersRequest(ctx context.Context, krakenPairs, originalPairs []string) ([]*entities.Price, error) {
	url := fmt.Sprintf("%s/Ticker?pair=%s", k.baseURL, strings.Join(krakenPairs, ","))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	requestStart := time.Now()
	resp, err := k.httpClient.Do(req)
	requestDuration := time.Since(requestStart)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("%w: context timeout/canceled", ErrRetryableRequest)
		}
		return nil, fmt.Errorf("%w: %v", ErrRetryableRequest, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Check for retryable HTTP status codes
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (server error)", ErrRetryableRequest, resp.StatusCode)
	}

	// CRITICAL FIX: Handle HTTP 429 (Too Many Requests) as retryable
	if resp.StatusCode == http.StatusTooManyRequests {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (rate limited by kraken)", ErrRetryableRequest, resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
		return nil, fmt.Errorf("%w: HTTP %d (client error)", ErrNonRetryable, resp.StatusCode)
	}

	var tickerResp KrakenTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %w", ErrRetryableRequest, err)
	}

	if len(tickerResp.Error) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrNonRetryable, strings.Join(tickerResp.Error, ", "))
	}

	prices := make([]*entities.Price, 0, len(originalPairs))

	// Crear mapeo de pares Kraken a pares originales
	krakenToOriginal := make(map[string]string)
	for i, originalPair := range originalPairs {
		if i < len(krakenPairs) {
			krakenToOriginal[krakenPairs[i]] = originalPair
		}
	}

	for returnedPair, tickerData := range tickerResp.Result {
		// Buscar el par original correspondiente
		originalPair := ""
		for krakenPair, origPair := range krakenToOriginal {
			if krakenPair == returnedPair || strings.Contains(returnedPair, krakenPair) {
				originalPair = origPair
				break
			}
		}

		if originalPair == "" {
			continue // Skip pairs we didn't request
		}

		price, err := tickerData.GetLastTradedPrice()
		if err != nil {
			return nil, fmt.Errorf("failed to get last traded price for %s: %w", originalPair, err)
		}

		prices = append(prices, entities.NewPrice(
			originalPair,
			price,
			tickerData.GetTimestamp(),
			tickerData.GetAge(),
		))
	}

	// Record successful external API call metrics
	if len(prices) > 0 {
		metrics.RecordExternalAPICall("kraken", "/Ticker", resp.StatusCode, float64(requestDuration.Nanoseconds())/1e6)
	}

	return prices, nil
}

var assetMap = map[string]string{
	"BTC": "XXBT", // Kraken usa XXBT para Bitcoin
	"ETH": "XETH", // Kraken usa XETH para Ethereum
	"LTC": "XLTC", // Kraken usa XLTC para Litecoin
	"XRP": "XXRP", // Kraken usa XXRP para Ripple
	"USD": "ZUSD", // Kraken usa ZUSD para USD
	"EUR": "ZEUR", // Kraken usa ZEUR para EUR
	"CHF": "CHF",
	"JPY": "JPY",
	"GBP": "GBP",
	"CAD": "CAD",
}
var krakenToFriendly map[string]string

func init() {
	krakenToFriendly = make(map[string]string)
	for k, v := range assetMap {
		krakenToFriendly[v] = k
	}
}

func toKrakenPair(symbol string) (string, error) {
	s := strings.ToUpper(symbol)
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid pair format, expected BASE/QUOTE: %s", symbol)
	}

	base, ok := assetMap[parts[0]]
	if !ok {
		return "", fmt.Errorf("unsupported base asset: %s", parts[0])
	}
	quote, ok := assetMap[parts[1]]
	if !ok {
		return "", fmt.Errorf("unsupported quote asset: %s", parts[1])
	}

	return base + quote, nil
}

func FromKrakenPair(pair string) (string, error) {
	s := strings.ToUpper(pair)

	for krBase, friendlyBase := range krakenToFriendly {
		if strings.HasPrefix(s, krBase) {
			krQuote := strings.TrimPrefix(s, krBase)
			friendlyQuote, ok := krakenToFriendly[krQuote]
			if !ok {
				return "", fmt.Errorf("activo quote no soportado: %s", krQuote)
			}
			return friendlyBase + "/" + friendlyQuote, nil
		}
	}

	return "", fmt.Errorf("base asset not supported in Kraken pair: %s", pair)
}
