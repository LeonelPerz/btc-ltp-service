package services

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/domain/interfaces"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/metrics"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultCacheTTL = 30 * time.Second // Default TTL for price cache entries
	CacheKeyPrefix  = "price:"         // Prefix for cache keys
)

// priceService implements the PriceService interface
type priceService struct {
	exchange       interfaces.Exchange
	cache          interfaces.Cache
	cacheTTL       time.Duration
	supportedPairs []string // Pares soportados para GetCachedPrices
}

// NewPriceService creates a new instance of the price service
func NewPriceService(exchange interfaces.Exchange, cache interfaces.Cache, supportedPairs []string) interfaces.PriceService {
	return &priceService{
		exchange:       exchange,
		cache:          cache,
		cacheTTL:       DefaultCacheTTL,
		supportedPairs: supportedPairs,
	}
}

// NewPriceServiceWithTTL creates an instance with custom TTL
func NewPriceServiceWithTTL(exchange interfaces.Exchange, cache interfaces.Cache, ttl time.Duration, supportedPairs []string) interfaces.PriceService {
	return &priceService{
		exchange:       exchange,
		cache:          cache,
		cacheTTL:       ttl,
		supportedPairs: supportedPairs,
	}
}

// GetLastPrice implements CACHE-ONLY strategy - NO fallback to exchange
// This method should ONLY be called for HTTP requests and MUST return from cache
func (s *priceService) GetLastPrice(ctx context.Context, pair string) (*entities.Price, error) {
	logging.Debug(ctx, "GetLastPrice: Retrieving price from cache ONLY", logging.Fields{
		"pair":      pair,
		"cache_key": s.cacheKey(pair),
		"source":    "cache_only",
	})

	// ONLY try to get from cache - NO fallback to exchange
	cachedPrice, err := s.getPriceFromCache(ctx, pair)
	if err != nil {
		// Cache miss - return error, NO fallback to exchange
		metrics.RecordCacheOperation("get", "miss")
		metrics.RecordPriceRequest(pair, false)

		logging.CacheOperation(ctx, "get", s.cacheKey(pair), false, logging.Fields{
			"pair":   pair,
			"error":  err.Error(),
			"policy": "cache_only_no_fallback",
		})

		return nil, fmt.Errorf("price not available in cache for %s (cache-only mode): %w", pair, err)
	}

	// Cache hit - return immediately
	metrics.RecordCacheOperation("get", "hit")
	metrics.RecordPriceRequest(pair, true)
	metrics.UpdateCurrentPrice(pair, cachedPrice.Amount)
	metrics.UpdatePriceAge(pair, cachedPrice.Age.Seconds())

	logging.CacheOperation(ctx, "get", s.cacheKey(pair), true, logging.Fields{
		"pair":   pair,
		"amount": cachedPrice.Amount,
		"age_ms": cachedPrice.Age.Milliseconds(),
		"policy": "cache_only_success",
	})

	return cachedPrice, nil
}

// RefreshPrices updates cache with fresh prices for multiple pairs
func (s *priceService) RefreshPrices(ctx context.Context, pairs []string) error {
	if len(pairs) == 0 {
		return nil
	}

	logging.Info(ctx, "Starting price refresh operation", logging.Fields{
		"pairs_count": len(pairs),
		"pairs":       pairs,
	})

	// Get prices from exchange (batch operation)
	exchangeStart := time.Now()
	prices, err := s.exchange.GetTickers(ctx, pairs)
	exchangeDuration := time.Since(exchangeStart)

	if err != nil {
		metrics.PriceRefreshesTotal.WithLabelValues("error").Inc()
		logging.ErrorWithError(ctx, "Failed to refresh prices from exchange", err, logging.Fields{
			"pairs_count":          len(pairs),
			"pairs":                pairs,
			"exchange_duration_ms": float64(exchangeDuration.Nanoseconds()) / 1e6,
		})
		return fmt.Errorf("failed to refresh prices from exchange: %w", err)
	}

	logging.Info(ctx, "Successfully retrieved prices from exchange", logging.Fields{
		"pairs_count":          len(pairs),
		"retrieved_count":      len(prices),
		"exchange_duration_ms": float64(exchangeDuration.Nanoseconds()) / 1e6,
	})

	// Cache all prices
	var errors []string
	successCount := 0
	for _, price := range prices {
		if err := s.cachePrice(ctx, price); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", price.Pair, err))
			logging.Warn(ctx, "Failed to cache individual price", logging.Fields{
				"pair":  price.Pair,
				"error": err.Error(),
			})
		} else {
			successCount++
			metrics.RecordCacheOperation("set", "success")
			metrics.UpdateCurrentPrice(price.Pair, price.Amount)
			metrics.UpdatePriceAge(price.Pair, 0) // Fresh price

			logging.CacheOperation(ctx, "set", s.cacheKey(price.Pair), true, logging.Fields{
				"pair":   price.Pair,
				"amount": price.Amount,
			})
		}
	}

	if len(errors) > 0 {
		metrics.PriceRefreshesTotal.WithLabelValues("error").Inc()
		logging.Error(ctx, "Failed to cache some prices during refresh", logging.Fields{
			"failed_count":  len(errors),
			"success_count": successCount,
			"errors":        errors,
		})
		return fmt.Errorf("failed to cache some prices: %s", strings.Join(errors, ", "))
	}

	metrics.PriceRefreshesTotal.WithLabelValues("success").Inc()
	logging.Info(ctx, "Successfully completed price refresh operation", logging.Fields{
		"pairs_count":   len(pairs),
		"cached_count":  len(prices),
		"success_count": successCount,
	})
	return nil
}

// GetCachedPrices returns all prices currently in cache for supported pairs
func (s *priceService) GetCachedPrices(ctx context.Context) ([]*entities.Price, error) {
	if len(s.supportedPairs) == 0 {
		logging.Warn(ctx, "No supported pairs configured for GetCachedPrices", nil)
		return []*entities.Price{}, nil
	}

	logging.Debug(ctx, "Retrieving cached prices for supported pairs", logging.Fields{
		"pairs_count": len(s.supportedPairs),
		"pairs":       s.supportedPairs,
	})

	var cachedPrices []*entities.Price

	for _, pair := range s.supportedPairs {
		if price, err := s.getPriceFromCache(ctx, pair); err == nil {
			cachedPrices = append(cachedPrices, price)
			logging.Debug(ctx, "Found cached price for pair", logging.Fields{
				"pair":   pair,
				"amount": price.Amount,
				"age_ms": price.Age.Milliseconds(),
			})
		} else {
			// Si no está en caché, simplemente continúa - esto es normal
			logging.Debug(ctx, "No cached price found for pair", logging.Fields{
				"pair": pair,
			})
		}
	}

	logging.Debug(ctx, "Retrieved cached prices", logging.Fields{
		"requested_count": len(s.supportedPairs),
		"found_count":     len(cachedPrices),
	})

	return cachedPrices, nil
}

// getPriceFromCache retrieves and deserializes a price from cache
func (s *priceService) getPriceFromCache(ctx context.Context, pair string) (*entities.Price, error) {
	key := s.cacheKey(pair)

	priceJSON, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var price entities.Price
	if err := json.Unmarshal([]byte(priceJSON), &price); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached price for %s: %w", pair, err)
	}

	// Update price age
	price.Age = time.Since(price.Timestamp)

	return &price, nil
}

// cachePrice serializes and stores a price in cache
func (s *priceService) cachePrice(ctx context.Context, price *entities.Price) error {
	key := s.cacheKey(price.Pair)

	priceJSON, err := json.Marshal(price)
	if err != nil {
		return fmt.Errorf("failed to marshal price for %s: %w", price.Pair, err)
	}

	return s.cache.Set(ctx, key, string(priceJSON), s.cacheTTL)
}

// cacheKey generates the cache key for a pair
func (s *priceService) cacheKey(pair string) string {
	return CacheKeyPrefix + strings.ToUpper(pair)
}
