package kraken

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"btc-ltp-service/internal/model"
)

const (
	KrakenAPIBaseURL = "https://api.kraken.com/0/public"
	TickerEndpoint   = "/Ticker"
	RequestTimeout   = 10 * time.Second
)

// Client represents a Kraken API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Kraken API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: RequestTimeout,
		},
		baseURL: KrakenAPIBaseURL,
	}
}

// GetTickerData retrieves ticker data for the specified pairs
func (c *Client) GetTickerData(pairs []string) (*model.KrakenResponse, error) {
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

	// Create and execute request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "btc-ltp-service/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	// Read and parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var krakenResp model.KrakenResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for API errors
	if len(krakenResp.Error) > 0 {
		return nil, fmt.Errorf("Kraken API error: %s", strings.Join(krakenResp.Error, ", "))
	}

	return &krakenResp, nil
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
