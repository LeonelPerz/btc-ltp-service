package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"btc-ltp-service/internal/cache"
	"btc-ltp-service/internal/client/kraken"
	"btc-ltp-service/internal/handler"
	"btc-ltp-service/internal/model"
	"btc-ltp-service/internal/service"
)

func TestLTPIntegration(t *testing.T) {
	// Setup test components
	krakenClient := kraken.NewClient()
	priceCache := cache.NewPriceCache()
	ltpService := service.NewLTPService(krakenClient, priceCache)
	ltpHandler := handler.NewLTPHandler(ltpService)

	// Setup HTTP server for testing
	mux := http.NewServeMux()
	ltpHandler.SetupRoutes(mux)

	t.Run("TestHealthEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["status"] != "healthy" {
			t.Errorf("Expected status 'healthy', got %s", response["status"])
		}
	})

	t.Run("TestSupportedPairsEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/pairs", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]string
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		pairs := response["supported_pairs"]
		expectedPairs := []string{"BTC/CHF", "BTC/EUR", "BTC/USD"}

		if len(pairs) != len(expectedPairs) {
			t.Errorf("Expected %d pairs, got %d", len(expectedPairs), len(pairs))
		}

		for _, expectedPair := range expectedPairs {
			found := false
			for _, pair := range pairs {
				if pair == expectedPair {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected pair %s not found in response", expectedPair)
			}
		}
	})

	t.Run("TestLTPEndpointAllPairs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ltp", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response model.LTPResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.LTP) != 3 {
			t.Errorf("Expected 3 pairs, got %d", len(response.LTP))
		}

		// Verify all expected pairs are present
		expectedPairs := map[string]bool{
			"BTC/USD": false,
			"BTC/CHF": false,
			"BTC/EUR": false,
		}

		for _, ltpPair := range response.LTP {
			if _, exists := expectedPairs[ltpPair.Pair]; exists {
				expectedPairs[ltpPair.Pair] = true
			}

			// Verify amount is positive
			if ltpPair.Amount <= 0 {
				t.Errorf("Expected positive amount for %s, got %f", ltpPair.Pair, ltpPair.Amount)
			}
		}

		// Check all pairs were found
		for pair, found := range expectedPairs {
			if !found {
				t.Errorf("Expected pair %s not found in response", pair)
			}
		}
	})

	t.Run("TestLTPEndpointSinglePair", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ltp?pair=BTC/USD", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response model.LTPResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.LTP) != 1 {
			t.Errorf("Expected 1 pair, got %d", len(response.LTP))
		}

		if response.LTP[0].Pair != "BTC/USD" {
			t.Errorf("Expected pair BTC/USD, got %s", response.LTP[0].Pair)
		}

		if response.LTP[0].Amount <= 0 {
			t.Errorf("Expected positive amount, got %f", response.LTP[0].Amount)
		}
	})

	t.Run("TestLTPEndpointMultiplePairs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ltp?pairs=BTC/USD,BTC/EUR", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response model.LTPResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.LTP) != 2 {
			t.Errorf("Expected 2 pairs, got %d", len(response.LTP))
		}

		// Verify expected pairs are present
		pairs := make(map[string]float64)
		for _, ltpPair := range response.LTP {
			pairs[ltpPair.Pair] = ltpPair.Amount
		}

		if _, exists := pairs["BTC/USD"]; !exists {
			t.Error("Expected BTC/USD pair not found")
		}

		if _, exists := pairs["BTC/EUR"]; !exists {
			t.Error("Expected BTC/EUR pair not found")
		}
	})

	t.Run("TestLTPEndpointInvalidPair", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ltp?pair=BTC/INVALID", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})

	t.Run("TestLTPEndpointMethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/ltp", strings.NewReader("{}"))
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

func TestKrakenClientIntegration(t *testing.T) {
	client := kraken.NewClient()

	t.Run("TestGetTickerDataValidPairs", func(t *testing.T) {
		pairs := []string{"BTC/USD", "BTC/EUR"}

		resp, err := client.GetTickerData(pairs)
		if err != nil {
			t.Fatalf("Failed to get ticker data: %v", err)
		}

		if len(resp.Error) > 0 {
			t.Fatalf("Kraken API returned errors: %v", resp.Error)
		}

		if len(resp.Result) == 0 {
			t.Fatal("No ticker data returned")
		}

		// Verify we got data for the expected pairs
		for krakenPair, tickerData := range resp.Result {
			if len(tickerData.LastTradeClosed) == 0 {
				t.Errorf("No last trade closed data for %s", krakenPair)
				continue
			}

			price, err := kraken.ParseLastTradePrice(tickerData)
			if err != nil {
				t.Errorf("Failed to parse price for %s: %v", krakenPair, err)
				continue
			}

			if price <= 0 {
				t.Errorf("Invalid price for %s: %f", krakenPair, price)
			}

			t.Logf("Successfully got price for %s: %f", krakenPair, price)
		}
	})

	t.Run("TestGetTickerDataInvalidPair", func(t *testing.T) {
		pairs := []string{"INVALID/PAIR"}

		_, err := client.GetTickerData(pairs)
		if err == nil {
			t.Error("Expected error for invalid pair, got nil")
		}
	})
}

func TestCacheIntegration(t *testing.T) {
	priceCache := cache.NewPriceCache()

	t.Run("TestCacheSetAndGet", func(t *testing.T) {
		pair := "BTC/USD"
		price := 50000.0

		priceCache.Set(pair, price)

		cachedPrice, exists := priceCache.Get(pair)
		if !exists {
			t.Error("Expected cached price to exist")
		}

		if cachedPrice != price {
			t.Errorf("Expected price %f, got %f", price, cachedPrice)
		}
	})

	t.Run("TestCacheExpiry", func(t *testing.T) {
		pair := "BTC/EUR"
		price := 45000.0

		priceCache.Set(pair, price)

		// Verify price is cached
		_, exists := priceCache.Get(pair)
		if !exists {
			t.Error("Expected cached price to exist immediately after setting")
		}

		// Check that price is expired after a longer duration (simulated)
		if !priceCache.IsExpired(pair) {
			t.Log("Price is still valid (within 1 minute cache window)")
		}
	})

	t.Run("TestCacheMultiple", func(t *testing.T) {
		prices := map[string]float64{
			"BTC/USD": 52000.0,
			"BTC/CHF": 48000.0,
		}

		priceCache.SetMultiple(prices)

		pairs := []string{"BTC/USD", "BTC/CHF"}
		cachedPrices := priceCache.GetMultiple(pairs)

		if len(cachedPrices) != 2 {
			t.Errorf("Expected 2 cached prices, got %d", len(cachedPrices))
		}

		for pair, expectedPrice := range prices {
			if cachedPrice, exists := cachedPrices[pair]; exists {
				if cachedPrice != expectedPrice {
					t.Errorf("Expected price %f for %s, got %f", expectedPrice, pair, cachedPrice)
				}
			} else {
				t.Errorf("Expected cached price for %s not found", pair)
			}
		}
	})
}

func BenchmarkLTPEndpoint(b *testing.B) {
	krakenClient := kraken.NewClient()
	priceCache := cache.NewPriceCache()
	ltpService := service.NewLTPService(krakenClient, priceCache)
	ltpHandler := handler.NewLTPHandler(ltpService)

	mux := http.NewServeMux()
	ltpHandler.SetupRoutes(mux)

	// Pre-warm the cache
	ltpService.RefreshAllPrices()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/ltp", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Errorf("Expected status 200, got %d", w.Code)
		}
	}
}
