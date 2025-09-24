package kraken

import (
	"btc-ltp-service/internal/domain/entities"
	cachepkg "btc-ltp-service/internal/infrastructure/repositories/cache"
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ===== BENCHMARKS - PRUEBAS DE RENDIMIENTO =====

func BenchmarkRestClient_GetTicker(b *testing.B) {
	mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.GetTicker(ctx, "BTC/USD")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRestClient_GetTickers(b *testing.B) {
	mockResponse := KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			"XXBTZUSD": {LastTradeClosed: []string{"50000.0", "1.0"}},
			"XETHZUSD": {LastTradeClosed: []string{"3000.0", "1.0"}},
			"XLTCZUSD": {LastTradeClosed: []string{"100.0", "1.0"}},
			"XXRPZUSD": {LastTradeClosed: []string{"0.5", "1.0"}},
			"ADAUSD":   {LastTradeClosed: []string{"0.3", "1.0"}},
		},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	pairs := []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD", "ADA/USD"}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.GetTickers(ctx, pairs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWebSocketClient_GetTicker(b *testing.B) {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	client := &WebSocketClient{
		url:           "ws://mock",
		subscriptions: map[string]bool{"BTC/USD": true},
		priceChannels: map[string]chan *entities.Price{
			"BTC/USD": make(chan *entities.Price, 1000),
		},
		cache:       cache,
		isConnected: true,
	}

	// Pre-llenar el canal con precios
	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	for i := 0; i < 1000; i++ {
		client.priceChannels["BTC/USD"] <- price
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.GetTicker(ctx, "BTC/USD")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKrakenTickerData_GetLastTradedPrice(b *testing.B) {
	tickerData := KrakenTickerData{
		LastTradeClosed: []string{"50000.123456789", "1.0"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := tickerData.GetLastTradedPrice()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageProcessing_TickerUpdate(b *testing.B) {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	client := &WebSocketClient{
		subscriptions: map[string]bool{"BTC/USD": true},
		priceChannels: map[string]chan *entities.Price{
			"BTC/USD": make(chan *entities.Price, 1000),
		},
		cache: cache,
	}

	tickerData := []interface{}{
		1, // channel ID
		map[string]interface{}{
			"c": []interface{}{"50000.0", "1.0"}, // last trade closed
		},
		"ticker",
		"XBT/USD",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := client.handleTickerUpdate(tickerData)
		if err != nil {
			b.Fatal(err)
		}

		// Drenar el canal para evitar bloqueos
		select {
		case <-client.priceChannels["BTC/USD"]:
		default:
		}
	}
}

func BenchmarkJSONParsing_KrakenResponse(b *testing.B) {
	responseJSON := `{
		"error": [],
		"result": {
			"XXBTZUSD": {
				"a": ["50001.0", "1", "1"],
				"b": ["49999.0", "1", "1"],
				"c": ["50000.0", "1.0"],
				"v": ["100", "200"],
				"p": ["50000.0", "50000.0"],
				"t": [10, 20],
				"l": ["49000.0", "49000.0"],
				"h": ["51000.0", "51000.0"],
				"o": "49500.0"
			}
		}
	}`

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var response KrakenTickerResponse
		err := json.Unmarshal([]byte(responseJSON), &response)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPairConversion_ToKrakenPair(b *testing.B) {
	pairs := []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD", "ADA/USD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pair := pairs[i%len(pairs)]
		_, err := toKrakenPair(pair)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPairConversion_FromKrakenPair(b *testing.B) {
	pairs := []string{"XXBTZUSD", "XETHZUSD", "XLTCZUSD", "XXRPZUSD", "ADAUSD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pair := pairs[i%len(pairs)]
		_, err := FromKrakenPair(pair)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPairConversion_ToWebSocketPair(b *testing.B) {
	pairs := []string{"BTC/USD", "ETH/USD", "LTC/USD", "XRP/USD", "ADA/USD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pair := pairs[i%len(pairs)]
		_, err := toWebSocketPair(pair)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPairConversion_FromWebSocketPair(b *testing.B) {
	pairs := []string{"XBT/USD", "ETH/USD", "LTC/USD", "XRP/USD", "ADA/USD"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pair := pairs[i%len(pairs)]
		_, err := fromWebSocketPair(pair)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCacheOperations_Set(b *testing.B) {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, price)
	}
}

func BenchmarkCacheOperations_Get(b *testing.B) {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	ctx := context.Background()
	_ = cache.Set(ctx, price)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "BTC/USD")
	}
}

// Benchmarks comparativos
func BenchmarkRestVsWebSocket_GetTicker(b *testing.B) {
	// Setup REST client
	mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	restClient := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	// Setup WebSocket client (mock)
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	wsClient := &WebSocketClient{
		subscriptions: map[string]bool{"BTC/USD": true},
		priceChannels: map[string]chan *entities.Price{
			"BTC/USD": make(chan *entities.Price, 1000),
		},
		cache:       cache,
		isConnected: true,
	}

	// Pre-llenar el canal WebSocket
	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	for i := 0; i < 1000; i++ {
		wsClient.priceChannels["BTC/USD"] <- price
	}

	ctx := context.Background()

	b.Run("REST", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := restClient.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WebSocket", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := wsClient.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark de concurrencia
func BenchmarkConcurrentRequests_REST(b *testing.B) {
	mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConcurrentRequests_WebSocket(b *testing.B) {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), time.Minute)

	client := &WebSocketClient{
		subscriptions: map[string]bool{"BTC/USD": true},
		priceChannels: map[string]chan *entities.Price{
			"BTC/USD": make(chan *entities.Price, 10000),
		},
		cache:       cache,
		isConnected: true,
	}

	// Pre-llenar el canal con muchos precios
	price := entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	for i := 0; i < 10000; i++ {
		client.priceChannels["BTC/USD"] <- price
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark de memoria y garbage collection
func BenchmarkMemoryAllocation_PriceCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = entities.NewPrice("BTC/USD", 50000.0, time.Now(), 0)
	}
}

func BenchmarkMemoryAllocation_TickerDataParsing(b *testing.B) {
	tickerData := KrakenTickerData{
		Ask:                 []string{"50001.0", "1", "1"},
		Bid:                 []string{"49999.0", "1", "1"},
		LastTradeClosed:     []string{"50000.0", "1.0"},
		Volume:              []string{"100", "200"},
		VolumeWeightedPrice: []string{"50000.0", "50000.0"},
		NumberOfTrades:      []interface{}{10, 20},
		Low:                 []string{"49000.0", "49000.0"},
		High:                []string{"51000.0", "51000.0"},
		OpeningPrice:        "49500.0",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := tickerData.GetLastTradedPrice()
		if err != nil {
			b.Fatal(err)
		}
		_ = tickerData.GetTimestamp()
		_ = tickerData.GetAge()
	}
}

// ===== NEW BENCHMARKS FOR 429/5XX SIMULATION =====

func BenchmarkRestClient_GetTicker_With429Simulation(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate 429 every 10 requests to measure impact of backoff
		if rand.Intn(10) == 0 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.GetTicker(ctx, "BTC/USD")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRestClient_GetTicker_With5xxSimulation(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate 5xx every 15 requests to measure impact of backoff
		if rand.Intn(15) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := client.GetTicker(ctx, "BTC/USD")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRestClient_BackoffPerformance_Comparison(b *testing.B) {
	b.Run("NoErrors", func(b *testing.B) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		client := &RestClient{
			baseURL:    server.URL,
			httpClient: &http.Client{Timeout: DefaultTimeout},
		}

		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("With429Errors", func(b *testing.B) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 5% chance of 429 error (less aggressive for benchmark completion)
			if rand.Intn(20) == 0 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResponse)
		}))
		defer server.Close()

		client := &RestClient{
			baseURL:    server.URL,
			httpClient: &http.Client{Timeout: DefaultTimeout},
		}

		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.GetTicker(ctx, "BTC/USD")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
