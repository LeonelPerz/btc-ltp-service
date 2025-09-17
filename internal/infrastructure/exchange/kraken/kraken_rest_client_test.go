package kraken

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Helper Functions
func createMockKrakenResponse(pair string, price string) KrakenTickerResponse {
	return KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			pair: {
				LastTradeClosed:     []string{price, "1.0"},
				Ask:                 []string{"50001.0", "1", "1"},
				Bid:                 []string{"49999.0", "1", "1"},
				Volume:              []string{"100", "200"},
				VolumeWeightedPrice: []string{"50000.0", "50000.0"},
				NumberOfTrades:      []interface{}{10, 20},
				Low:                 []string{"49000.0", "49000.0"},
				High:                []string{"51000.0", "51000.0"},
				OpeningPrice:        "49500.0",
			},
		},
	}
}

func createMockServer(statusCode int, response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if response != nil {
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
}

// ===== CASOS DE ÉXITO - FUNCIONAMIENTO NORMAL =====

func TestNewRestClient_DefaultConfiguration(t *testing.T) {
	client := NewRestClient()

	assert.NotNil(t, client)
	assert.Equal(t, KrakenAPIBaseURL, client.baseURL)
	assert.Equal(t, DefaultTimeout, client.httpClient.Timeout)
}

func TestNewRestClientWithConfig_CustomConfiguration(t *testing.T) {
	cfg := config.KrakenConfig{
		RestURL: "https://custom-api.kraken.com/0/public",
		Timeout: 5 * time.Second,
	}

	client := NewRestClientWithConfig(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg.RestURL, client.baseURL)
	assert.Equal(t, cfg.Timeout, client.httpClient.Timeout)
}

func TestRestClient_GetTicker_Success(t *testing.T) {
	mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	price, err := client.GetTicker(ctx, "BTC/USD")

	require.NoError(t, err)
	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Equal(t, 50000.0, price.Amount)
	assert.WithinDuration(t, time.Now(), price.Timestamp, time.Second)
}

func TestRestClient_GetTickers_Success(t *testing.T) {
	mockResponse := KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			"XXBTZUSD": {
				LastTradeClosed: []string{"50000.0", "1.0"},
				Ask:             []string{"50001.0", "1", "1"},
				Bid:             []string{"49999.0", "1", "1"},
			},
			"XETHZUSD": {
				LastTradeClosed: []string{"3000.0", "1.0"},
				Ask:             []string{"3001.0", "1", "1"},
				Bid:             []string{"2999.0", "1", "1"},
			},
		},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	prices, err := client.GetTickers(ctx, []string{"BTC/USD", "ETH/USD"})

	require.NoError(t, err)
	require.Len(t, prices, 2)

	// Verificar que tenemos ambos pares
	pairMap := make(map[string]*entities.Price)
	for _, price := range prices {
		pairMap[price.Pair] = price
	}

	assert.Contains(t, pairMap, "BTC/USD")
	assert.Contains(t, pairMap, "ETH/USD")
	assert.Equal(t, 50000.0, pairMap["BTC/USD"].Amount)
	assert.Equal(t, 3000.0, pairMap["ETH/USD"].Amount)
}

func TestRestClient_GetTickers_EmptyPairsList(t *testing.T) {
	client := NewRestClient()

	ctx := context.Background()
	prices, err := client.GetTickers(ctx, []string{})

	require.NoError(t, err)
	assert.Empty(t, prices)
}

// ===== CASOS DE ERROR - MANEJO DE ERRORES Y EXCEPCIONES =====

func TestRestClient_GetTicker_InvalidPair(t *testing.T) {
	client := NewRestClient()

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "INVALID")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert pair to kraken pair")
}

func TestRestClient_GetTicker_NetworkError(t *testing.T) {
	client := &RestClient{
		baseURL:    "http://nonexistent-server.com",
		httpClient: &http.Client{Timeout: 100 * time.Millisecond},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ticker after retries")
}

func TestRestClient_GetTicker_HTTPStatusError_4xx(t *testing.T) {
	server := createMockServer(http.StatusBadRequest, nil)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
}

func TestRestClient_GetTicker_HTTPStatusError_5xx(t *testing.T) {
	server := createMockServer(http.StatusInternalServerError, nil)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ticker after retries")
}

func TestRestClient_GetTicker_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ticker after retries")
}

func TestRestClient_GetTicker_KrakenAPIError(t *testing.T) {
	mockResponse := KrakenTickerResponse{
		Error:  []string{"EQuery:Invalid asset pair"},
		Result: map[string]KrakenTickerData{},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EQuery:Invalid asset pair")
}

func TestRestClient_GetTicker_EmptyResponse(t *testing.T) {
	mockResponse := KrakenTickerResponse{
		Error:  []string{},
		Result: map[string]KrakenTickerData{},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ticker data found")
}

// ===== EDGE CASES - CASOS LÍMITE Y SITUACIONES EXTREMAS =====

func TestRestClient_GetTicker_EmptyPair(t *testing.T) {
	client := NewRestClient()

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "")

	assert.Error(t, err)
}

func TestRestClient_GetTicker_InvalidTickerData(t *testing.T) {
	mockResponse := KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			"XXBTZUSD": {
				LastTradeClosed: []string{}, // Empty price data
			},
		},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticker data")
}

func TestRestClient_GetTickers_MixedValidInvalidPairs(t *testing.T) {
	client := NewRestClient()

	ctx := context.Background()
	_, err := client.GetTickers(ctx, []string{"BTC/USD", "INVALID", "ETH/USD"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert pair to kraken pair")
}

// ===== CONCURRENCIA - ACCESO CONCURRENTE Y THREAD-SAFETY =====

func TestRestClient_GetTicker_ConcurrentRequests(t *testing.T) {
	mockResponse := createMockKrakenResponse("XXBTZUSD", "50000.0")
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *entities.Price, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			price, err := client.GetTicker(ctx, "BTC/USD")
			if err != nil {
				errors <- err
				return
			}
			results <- price
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Verificar que no hubo errores
	for err := range errors {
		t.Errorf("Unexpected error in concurrent request: %v", err)
	}

	// Verificar que todas las respuestas son correctas
	count := 0
	for price := range results {
		assert.Equal(t, "BTC/USD", price.Pair)
		assert.Equal(t, 50000.0, price.Amount)
		count++
	}
	assert.Equal(t, numGoroutines, count)
}

func TestRestClient_GetTickers_ConcurrentRequests(t *testing.T) {
	mockResponse := KrakenTickerResponse{
		Error: []string{},
		Result: map[string]KrakenTickerData{
			"XXBTZUSD": {LastTradeClosed: []string{"50000.0", "1.0"}},
			"XETHZUSD": {LastTradeClosed: []string{"3000.0", "1.0"}},
		},
	}
	server := createMockServer(http.StatusOK, mockResponse)
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan []*entities.Price, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			prices, err := client.GetTickers(ctx, []string{"BTC/USD", "ETH/USD"})
			if err != nil {
				errors <- err
				return
			}
			results <- prices
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Verificar que no hubo errores
	for err := range errors {
		t.Errorf("Unexpected error in concurrent request: %v", err)
	}

	// Verificar que todas las respuestas son correctas
	count := 0
	for prices := range results {
		assert.Len(t, prices, 2)
		count++
	}
	assert.Equal(t, numGoroutines, count)
}

// ===== TIMEOUTS Y CANCELACIONES - MANEJO DE CONTEXTOS =====

func TestRestClient_GetTicker_ContextCanceled(t *testing.T) {
	// Crear un servidor que responde lentamente
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(createMockKrakenResponse("XXBTZUSD", "50000.0"))
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestRestClient_GetTicker_ContextTimeout(t *testing.T) {
	// Crear un servidor que responde lentamente
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(createMockKrakenResponse("XXBTZUSD", "50000.0"))
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestRestClient_GetTickers_ContextTimeout(t *testing.T) {
	// Crear un servidor que responde lentamente
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(KrakenTickerResponse{
			Error: []string{},
			Result: map[string]KrakenTickerData{
				"XXBTZUSD": {LastTradeClosed: []string{"50000.0", "1.0"}},
			},
		})
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetTickers(ctx, []string{"BTC/USD"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

// ===== LÓGICA DE REINTENTOS =====

func TestRestClient_GetTicker_RetrySuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(createMockKrakenResponse("XXBTZUSD", "50000.0"))
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	price, err := client.GetTicker(ctx, "BTC/USD")

	require.NoError(t, err)
	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Equal(t, 50000.0, price.Amount)
	assert.Equal(t, 2, callCount) // Verificar que se hicieron 2 llamadas
}

func TestRestClient_GetTicker_RetryExhausted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ticker after retries")
	assert.Equal(t, MaxRetries, callCount) // Verificar que se agotaron los reintentos
}

func TestRestClient_GetTicker_NoRetryOnNonRetryableError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest) // Error 4xx no reintentable
	}))
	defer server.Close()

	client := &RestClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // Solo una llamada, sin reintentos
}

func TestRestClient_isRetryableError_RetryableErrors(t *testing.T) {
	client := NewRestClient()

	assert.True(t, client.isRetryableError(ErrRetryableRequest))
	assert.True(t, client.isRetryableError(fmt.Errorf("wrapped: %w", ErrRetryableRequest)))
}

func TestRestClient_isRetryableError_NonRetryableErrors(t *testing.T) {
	client := NewRestClient()

	assert.False(t, client.isRetryableError(ErrNonRetryable))
	assert.False(t, client.isRetryableError(fmt.Errorf("wrapped: %w", ErrNonRetryable)))
	assert.False(t, client.isRetryableError(fmt.Errorf("some other error")))
}

// ===== PAIR CONVERSION TESTS =====

func TestToKrakenPair_ValidPairs(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"BTC/USD", "XXBTZUSD"},
		{"ETH/USD", "XETHZUSD"},
		{"BTC/EUR", "XXBTZEUR"},
		{"ETH/EUR", "XETHZEUR"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := toKrakenPair(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToKrakenPair_InvalidPairs(t *testing.T) {
	testCases := []string{
		"INVALID",
		"BTC",
		"BTC/",
		"/USD",
		"",
		"BTC/INVALID",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := toKrakenPair(tc)
			assert.Error(t, err)
		})
	}
}

func TestFromKrakenPair_ValidPairs(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"XXBTZUSD", "BTC/USD"},
		{"XETHZUSD", "ETH/USD"},
		{"XXBTZEUR", "BTC/EUR"},
		{"XETHZEUR", "ETH/EUR"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := FromKrakenPair(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFromKrakenPair_InvalidPairs(t *testing.T) {
	testCases := []string{
		"INVALID",
		"",
		"XBT",
		"ZUSD",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := FromKrakenPair(tc)
			assert.Error(t, err)
		})
	}
}
