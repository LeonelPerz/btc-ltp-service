package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"btc-ltp-service/internal/metrics"
	"btc-ltp-service/internal/model"
)

const (
	KrakenAPIBaseURL = "https://api.kraken.com/0/public"
	TickerEndpoint   = "/Ticker"
	DefaultTimeout   = 10 * time.Second
	MaxRetries       = 3
	BaseBackoffDelay = 1 * time.Second
)

// Client represents a Kraken API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	timeout    time.Duration
}

// NewClient creates a new Kraken API client with default timeout
func NewClient() *Client {
	return NewClientWithTimeout(DefaultTimeout)
}

// NewClientWithTimeout creates a new Kraken API client with custom timeout
func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: KrakenAPIBaseURL,
		timeout: timeout,
	}
}

// GetTickerData retrieves ticker data for the specified pairs with context, timeout and retries
func (c *Client) GetTickerData(pairs []string) (*model.KrakenResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	return c.GetTickerDataWithContext(ctx, pairs)
}

// GetTickerDataWithContext retrieves ticker data with context support and retry logic
func (c *Client) GetTickerDataWithContext(ctx context.Context, pairs []string) (*model.KrakenResponse, error) {
	start := time.Now()

	if len(pairs) == 0 {
		return nil, fmt.Errorf("no pairs specified")
	}

	// Convert standard pair names to Kraken format
	krakenPairs := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if krakenPair, exists := model.SupportedPairs[pair]; exists {
			krakenPairs = append(krakenPairs, krakenPair)
		} else {
			return nil, fmt.Errorf("unsupported pair: %s", pair)
		}
	}

	// Build URL with query parameters
	u, err := url.Parse(c.baseURL + TickerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	q := u.Query()
	q.Set("pair", strings.Join(krakenPairs, ","))
	u.RawQuery = q.Encode()

	var lastErr error

	// Retry logic with exponential backoff
	for attempt := 0; attempt < MaxRetries; attempt++ {
		// Create request with context
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "btc-ltp-service/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute request (attempt %d/%d): %w", attempt+1, MaxRetries, err)
			metrics.RecordKrakenError("network_error")

			// Check if this is a retryable error
			if !isRetryableError(err, 0) {
				duration := time.Since(start)
				metrics.RecordKrakenRequest(0, duration) // 0 indicates network error
				return nil, lastErr
			}

			if attempt > 0 {
				metrics.RecordKrakenRetry()
			}

			if attempt < MaxRetries-1 {
				backoffDelay := time.Duration(math.Pow(2, float64(attempt))) * BaseBackoffDelay
				select {
				case <-time.After(backoffDelay):
					continue
				case <-ctx.Done():
					duration := time.Since(start)
					metrics.RecordKrakenRequest(0, duration)
					return nil, ctx.Err()
				}
			}
			continue
		}

		// Check HTTP status code
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("API request failed with server error %d (attempt %d/%d)", resp.StatusCode, attempt+1, MaxRetries)
			metrics.RecordKrakenError("server_error")

			if attempt > 0 {
				metrics.RecordKrakenRetry()
			}

			if attempt < MaxRetries-1 {
				backoffDelay := time.Duration(math.Pow(2, float64(attempt))) * BaseBackoffDelay
				select {
				case <-time.After(backoffDelay):
					continue
				case <-ctx.Done():
					duration := time.Since(start)
					metrics.RecordKrakenRequest(resp.StatusCode, duration)
					return nil, ctx.Err()
				}
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			duration := time.Since(start)
			metrics.RecordKrakenRequest(resp.StatusCode, duration)
			metrics.RecordKrakenError("api_error")
			return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
		}

		// Read and parse response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var krakenResp model.KrakenResponse
		if err := json.Unmarshal(body, &krakenResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Check for API errors
		if len(krakenResp.Error) > 0 {
			duration := time.Since(start)
			metrics.RecordKrakenRequest(resp.StatusCode, duration)
			metrics.RecordKrakenError("kraken_api_error")
			return nil, fmt.Errorf("Kraken API error: %s", strings.Join(krakenResp.Error, ", "))
		}

		// Success - record metrics
		duration := time.Since(start)
		metrics.RecordKrakenRequest(resp.StatusCode, duration)

		return &krakenResp, nil
	}

	duration := time.Since(start)
	metrics.RecordKrakenRequest(0, duration) // 0 indicates failure
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError determines if an error is worth retrying
func isRetryableError(err error, statusCode int) bool {
	// Context errors are not retryable
	if err == context.DeadlineExceeded || err == context.Canceled {
		return false
	}

	// Server errors (5xx) are retryable
	if statusCode >= 500 {
		return true
	}

	// Network errors are generally retryable
	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"no such host",
		"timeout",
		"temporary failure",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}

	return false
}

// ParseLastTradePrice extracts the last traded price from Kraken ticker data
func ParseLastTradePrice(tickerData model.KrakenTickerData) (float64, error) {
	if len(tickerData.LastTradeClosed) == 0 {
		return 0, fmt.Errorf("no last trade closed data available")
	}

	priceStr := tickerData.LastTradeClosed[0]
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price '%s': %w", priceStr, err)
	}

	return price, nil
}
