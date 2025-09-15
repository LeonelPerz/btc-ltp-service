package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"btc-ltp-service/internal/logger"
)

// KrakenAssetPairsResponse represents the response from Kraken's AssetPairs endpoint
type KrakenAssetPairsResponse struct {
	Error  []string                       `json:"error"`
	Result map[string]KrakenAssetPairInfo `json:"result"`
}

// KrakenAssetPairInfo represents information about a trading pair from Kraken
type KrakenAssetPairInfo struct {
	Altname           string      `json:"altname"`             // Alternative pair name
	WSName            string      `json:"wsname"`              // WebSocket pair name
	AClassBase        string      `json:"aclass_base"`         // Asset class of base component
	Base              string      `json:"base"`                // Asset id of base component
	AClassQuote       string      `json:"aclass_quote"`        // Asset class of quote component
	Quote             string      `json:"quote"`               // Asset id of quote component
	Lot               string      `json:"lot"`                 // Volume lot size
	PairDecimals      int         `json:"pair_decimals"`       // Scaling decimal places for pair
	LotDecimals       int         `json:"lot_decimals"`        // Scaling decimal places for volume
	LotMultiplier     int         `json:"lot_multiplier"`      // Amount to multiply lot volume by to get currency volume
	LeverageBuy       []int       `json:"leverage_buy"`        // Array of leverage amounts available when buying
	LeverageSell      []int       `json:"leverage_sell"`       // Array of leverage amounts available when selling
	Fees              [][]float64 `json:"fees"`                // Fee schedule array
	FeesMaker         [][]float64 `json:"fees_maker"`          // Maker fee schedule array
	FeeVolumeCurrency string      `json:"fee_volume_currency"` // Volume discount currency
	MarginCall        int         `json:"margin_call"`         // Margin call level
	MarginStop        int         `json:"margin_stop"`         // Stop-out/liquidation margin level
	OrderMin          string      `json:"ordermin"`            // Minimum order volume for pair
}

// PairMapper handles mapping between different pair nomenclatures
type PairMapper struct {
	mu             sync.RWMutex
	standardToREST map[string]string // Standard format (BTC/USD) to REST format (XXBTZUSD)
	standardToWS   map[string]string // Standard format (BTC/USD) to WebSocket format (XBT/USD)
	restToStandard map[string]string // REST format to Standard format
	wsToStandard   map[string]string // WebSocket format to Standard format
	lastUpdate     time.Time
	updateInterval time.Duration
	krakenBaseURL  string
	client         *http.Client
	initialized    bool
}

// NewPairMapper creates a new PairMapper instance
func NewPairMapper(krakenBaseURL string) *PairMapper {
	if krakenBaseURL == "" {
		krakenBaseURL = "https://api.kraken.com"
	}

	return &PairMapper{
		standardToREST: make(map[string]string),
		standardToWS:   make(map[string]string),
		restToStandard: make(map[string]string),
		wsToStandard:   make(map[string]string),
		updateInterval: 24 * time.Hour, // Update once per day
		krakenBaseURL:  krakenBaseURL,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

// Initialize fetches pair mappings from Kraken API and initializes the mapper
func (pm *PairMapper) Initialize(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if err := pm.fetchPairMappings(ctx); err != nil {
		return fmt.Errorf("failed to fetch pair mappings: %w", err)
	}

	pm.initialized = true
	logger.GetLogger().Info("PairMapper initialized successfully")
	return nil
}

// fetchPairMappings fetches pair information from Kraken's AssetPairs endpoint
func (pm *PairMapper) fetchPairMappings(ctx context.Context) error {
	url := fmt.Sprintf("%s/0/public/AssetPairs", pm.krakenBaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := pm.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch asset pairs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var krakenResp KrakenAssetPairsResponse
	if err := json.Unmarshal(body, &krakenResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(krakenResp.Error) > 0 {
		return fmt.Errorf("kraken API error: %v", krakenResp.Error)
	}

	// Clear existing mappings
	pm.standardToREST = make(map[string]string)
	pm.standardToWS = make(map[string]string)
	pm.restToStandard = make(map[string]string)
	pm.wsToStandard = make(map[string]string)

	// Build mappings
	for restName, pairInfo := range krakenResp.Result {
		if pairInfo.WSName == "" {
			continue // Skip pairs without WebSocket support
		}

		// Create standard format (e.g., BTC/USD, ETH/EUR)
		standardName := pm.createStandardName(pairInfo.Base, pairInfo.Quote)

		// Store mappings
		pm.standardToREST[standardName] = restName
		pm.standardToWS[standardName] = pairInfo.WSName
		pm.restToStandard[restName] = standardName
		pm.wsToStandard[pairInfo.WSName] = standardName

		logger.GetLogger().WithFields(map[string]interface{}{
			"standard": standardName,
			"rest":     restName,
			"ws":       pairInfo.WSName,
		}).Debug("Added pair mapping")
	}

	pm.lastUpdate = time.Now()

	logger.GetLogger().WithField("pairs_count", len(pm.standardToREST)).Info("Pair mappings updated from Kraken API")

	return nil
}

// createStandardName creates a standard pair name from base and quote assets
func (pm *PairMapper) createStandardName(base, quote string) string {
	// Normalize common asset names
	standardBase := pm.normalizeAssetName(base)
	standardQuote := pm.normalizeAssetName(quote)

	return fmt.Sprintf("%s/%s", standardBase, standardQuote)
}

// normalizeAssetName converts Kraken asset names to standard format
func (pm *PairMapper) normalizeAssetName(asset string) string {
	switch asset {
	case "XXBT", "XBT":
		return "BTC"
	case "XETH":
		return "ETH"
	case "XLTC":
		return "LTC"
	case "ZUSD":
		return "USD"
	case "ZEUR":
		return "EUR"
	case "ZGBP":
		return "GBP"
	case "ZCAD":
		return "CAD"
	case "ZJPY":
		return "JPY"
	case "ZAUD":
		return "AUD"
	case "ZCHF":
		return "CHF"
	default:
		// Remove leading X or Z for other assets
		if len(asset) > 1 && (asset[0] == 'X' || asset[0] == 'Z') {
			return asset[1:]
		}
		return asset
	}
}

// ToRESTFormat converts standard pair name to Kraken REST API format
func (pm *PairMapper) ToRESTFormat(standardPair string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.initialized {
		return "", fmt.Errorf("pair mapper not initialized")
	}

	restPair, exists := pm.standardToREST[standardPair]
	if !exists {
		return "", fmt.Errorf("no REST mapping found for pair: %s", standardPair)
	}

	return restPair, nil
}

// ToWSFormat converts standard pair name to Kraken WebSocket format
func (pm *PairMapper) ToWSFormat(standardPair string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.initialized {
		return "", fmt.Errorf("pair mapper not initialized")
	}

	wsPair, exists := pm.standardToWS[standardPair]
	if !exists {
		return "", fmt.Errorf("no WebSocket mapping found for pair: %s", standardPair)
	}

	return wsPair, nil
}

// ToStandardFromREST converts REST API pair name to standard format
func (pm *PairMapper) ToStandardFromREST(restPair string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.initialized {
		return "", fmt.Errorf("pair mapper not initialized")
	}

	standardPair, exists := pm.restToStandard[restPair]
	if !exists {
		return "", fmt.Errorf("no standard mapping found for REST pair: %s", restPair)
	}

	return standardPair, nil
}

// ToStandardFromWS converts WebSocket pair name to standard format
func (pm *PairMapper) ToStandardFromWS(wsPair string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.initialized {
		return "", fmt.Errorf("pair mapper not initialized")
	}

	standardPair, exists := pm.wsToStandard[wsPair]
	if !exists {
		return "", fmt.Errorf("no standard mapping found for WebSocket pair: %s", wsPair)
	}

	return standardPair, nil
}

// GetSupportedStandardPairs returns all supported pairs in standard format
func (pm *PairMapper) GetSupportedStandardPairs() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pairs := make([]string, 0, len(pm.standardToREST))
	for standardPair := range pm.standardToREST {
		pairs = append(pairs, standardPair)
	}

	return pairs
}

// IsSupported checks if a standard pair is supported
func (pm *PairMapper) IsSupported(standardPair string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	_, exists := pm.standardToREST[standardPair]
	return exists
}

// UpdateIfNeeded updates pair mappings if needed (based on update interval)
func (pm *PairMapper) UpdateIfNeeded(ctx context.Context) error {
	pm.mu.RLock()
	needsUpdate := time.Since(pm.lastUpdate) > pm.updateInterval
	pm.mu.RUnlock()

	if needsUpdate {
		logger.GetLogger().Info("Updating pair mappings from Kraken API")
		return pm.Initialize(ctx)
	}

	return nil
}

// IsInitialized returns whether the pair mapper has been initialized
func (pm *PairMapper) IsInitialized() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.initialized
}
