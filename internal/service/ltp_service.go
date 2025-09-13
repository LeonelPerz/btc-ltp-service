package service

import (
	"fmt"
	"log"
	"sort"
	"time"

	"btc-ltp-service/internal/cache"
	"btc-ltp-service/internal/client/kraken"
	"btc-ltp-service/internal/metrics"
	"btc-ltp-service/internal/model"
)

// LTPService handles Last Traded Price operations
type LTPService struct {
	krakenClient *kraken.Client
	priceCache   cache.Cache
}

// NewLTPService creates a new LTP service instance
func NewLTPService(krakenClient *kraken.Client, priceCache cache.Cache) *LTPService {
	return &LTPService{
		krakenClient: krakenClient,
		priceCache:   priceCache,
	}
}

// GetLTP retrieves Last Traded Prices for the specified pairs
// If pairs is empty, it returns data for all supported pairs
func (s *LTPService) GetLTP(pairs []string) (*model.LTPResponse, error) {
	// If no pairs specified, use all supported pairs
	if len(pairs) == 0 {
		pairs = s.getAllSupportedPairs()
	}

	// Validate requested pairs
	if err := s.validatePairs(pairs); err != nil {
		return nil, err
	}

	// Try to get cached prices first
	cachedPrices, err := s.priceCache.GetMultiple(pairs)
	if err != nil {
		log.Printf("Warning: failed to get cached prices: %v", err)
		cachedPrices = make(map[string]float64)
	}

	// Determine which pairs need fresh data
	expiredPairs, err := s.priceCache.GetExpiredPairs(pairs)
	if err != nil {
		log.Printf("Warning: failed to get expired pairs: %v", err)
		// If we can't determine expired pairs, fetch all
		expiredPairs = pairs
	}

	// Fetch fresh data for expired pairs
	if len(expiredPairs) > 0 {
		if err := s.fetchAndCachePrices(expiredPairs); err != nil {
			log.Printf("Warning: failed to fetch fresh prices for %v: %v", expiredPairs, err)
			// Continue with cached data if available
		} else {
			// Update cached prices with fresh data
			freshPrices, err := s.priceCache.GetMultiple(expiredPairs)
			if err != nil {
				log.Printf("Warning: failed to get fresh prices: %v", err)
			} else {
				for pair, price := range freshPrices {
					cachedPrices[pair] = price
				}
			}
		}
	}

	// Build response
	response := &model.LTPResponse{
		LTP: make([]model.LTPPair, 0, len(pairs)),
	}

	// Add prices for requested pairs
	for _, pair := range pairs {
		if price, exists := cachedPrices[pair]; exists {
			response.LTP = append(response.LTP, model.LTPPair{
				Pair:   pair,
				Amount: price,
			})
		} else {
			return nil, fmt.Errorf("price not available for pair: %s", pair)
		}
	}

	// Sort results by pair name for consistent output
	sort.Slice(response.LTP, func(i, j int) bool {
		return response.LTP[i].Pair < response.LTP[j].Pair
	})

	return response, nil
}

// fetchAndCachePrices fetches fresh prices from Kraken and updates cache
func (s *LTPService) fetchAndCachePrices(pairs []string) error {
	krakenResp, err := s.krakenClient.GetTickerData(pairs)
	if err != nil {
		return fmt.Errorf("failed to fetch ticker data: %w", err)
	}

	prices := make(map[string]float64)

	// Process each pair in the response
	for krakenPair, tickerData := range krakenResp.Result {
		// Convert Kraken pair name to standard format
		standardPair, exists := model.KrakenToStandardPair[krakenPair]
		if !exists {
			log.Printf("Warning: unknown Kraken pair %s", krakenPair)
			continue
		}

		// Parse the last traded price
		price, err := kraken.ParseLastTradePrice(tickerData)
		if err != nil {
			log.Printf("Warning: failed to parse price for %s: %v", standardPair, err)
			continue
		}

		prices[standardPair] = price
	}

	// Update cache with fresh prices
	if len(prices) > 0 {
		if err := s.priceCache.SetMultiple(prices); err != nil {
			log.Printf("Warning: failed to update cache: %v", err)
		} else {
			log.Printf("Updated cache with %d fresh prices", len(prices))

			// Update current price metrics
			for pair, price := range prices {
				metrics.UpdateCurrentPrice(pair, price)
			}
		}
	}

	return nil
}

// validatePairs checks if all requested pairs are supported
func (s *LTPService) validatePairs(pairs []string) error {
	for _, pair := range pairs {
		if _, exists := model.SupportedPairs[pair]; !exists {
			return fmt.Errorf("unsupported trading pair: %s", pair)
		}
	}
	return nil
}

// getAllSupportedPairs returns a slice of all supported trading pairs
func (s *LTPService) getAllSupportedPairs() []string {
	pairs := make([]string, 0, len(model.SupportedPairs))
	for pair := range model.SupportedPairs {
		pairs = append(pairs, pair)
	}

	// Sort for consistent ordering
	sort.Strings(pairs)
	return pairs
}

// RefreshAllPrices forcefully refreshes prices for all supported pairs
func (s *LTPService) RefreshAllPrices() error {
	start := time.Now()
	allPairs := s.getAllSupportedPairs()

	err := s.fetchAndCachePrices(allPairs)
	duration := time.Since(start)

	if err != nil {
		metrics.RecordPriceRefresh("error", duration)
		return err
	}

	metrics.RecordPriceRefresh("success", duration)
	return nil
}

// GetSupportedPairs returns the list of supported trading pairs
func (s *LTPService) GetSupportedPairs() []string {
	return s.getAllSupportedPairs()
}
