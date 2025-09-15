package kraken

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"btc-ltp-service/internal/config"
	"btc-ltp-service/internal/logger"
	"btc-ltp-service/internal/metrics"
	"btc-ltp-service/internal/model"
	"btc-ltp-service/internal/pairs"
	"btc-ltp-service/internal/ratelimit"
)

// HybridClient combines WebSocket and REST clients with automatic fallback
type HybridClient struct {
	restClient   *Client
	wsClient     *WebSocketClient
	config       config.KrakenConfig
	wsEnabled    bool
	mu           sync.RWMutex
	fallbackMode bool
	lastWSUpdate time.Time
	pairMapper   *pairs.PairMapper
}

// NewHybridClient creates a new hybrid client that uses WebSocket with REST fallback
func NewHybridClient(cfg config.KrakenConfig) *HybridClient {
	// Create REST client with rate limiting from config
	restClient := createRestClientFromConfig(cfg)

	// Create PairMapper with Kraken base URL from config
	var krakenBaseURL string
	if cfg.BaseURL != "" {
		krakenBaseURL = cfg.BaseURL
	} else {
		krakenBaseURL = "https://api.kraken.com"
	}
	pairMapper := pairs.NewPairMapper(krakenBaseURL)

	// Initialize PairMapper - if this fails, we'll log the error but continue
	// The clients will fall back to legacy mappings
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pairMapper.Initialize(ctx); err != nil {
		logger.GetLogger().WithField("error", err.Error()).Error("Failed to initialize PairMapper, using legacy mappings")
	} else {
		logger.GetLogger().Info("PairMapper initialized successfully")
	}

	var wsClient *WebSocketClient

	// Create WebSocket client if enabled
	if cfg.WebSocketEnabled {
		wsClient = NewWebSocketClient(
			cfg.WebSocketURL,
			cfg.WebSocketTimeout,
			cfg.MaxReconnectTries,
			cfg.ReconnectDelay,
			pairMapper,
		)
	}

	client := &HybridClient{
		restClient: restClient,
		wsClient:   wsClient,
		config:     cfg,
		wsEnabled:  cfg.WebSocketEnabled,
		pairMapper: pairMapper,
	}

	// Set up WebSocket price update callback
	if wsClient != nil {
		wsClient.SetPriceUpdateCallback(client.handleWebSocketPriceUpdate)
	}

	return client
}

// NewHybridClientWithTimeout creates a hybrid client with custom timeout (for compatibility)
func NewHybridClientWithTimeout(timeout time.Duration) *HybridClient {
	cfg := config.KrakenConfig{
		Timeout:           timeout,
		WebSocketEnabled:  false, // Disable WebSocket for compatibility mode
		WebSocketURL:      "wss://ws.kraken.com/",
		WebSocketTimeout:  30 * time.Second,
		ReconnectDelay:    5 * time.Second,
		MaxReconnectTries: 5,
		BaseURL:           "https://api.kraken.com",
	}

	return NewHybridClient(cfg)
}

// Start initializes WebSocket connection if enabled
func (h *HybridClient) Start(pairs []string) error {
	if !h.wsEnabled || h.wsClient == nil {
		logger.GetLogger().Info("WebSocket disabled, using REST only")
		return nil
	}

	logger.GetLogger().Info("Starting WebSocket connection for real-time price updates")

	// Connect to WebSocket
	if err := h.wsClient.Connect(); err != nil {
		logger.GetLogger().WithField("error", err.Error()).Error("Failed to connect WebSocket, falling back to REST")
		h.setFallbackMode(true)
		return nil // Don't return error, just use REST fallback
	}

	// Subscribe to ticker data
	if err := h.wsClient.Subscribe(pairs); err != nil {
		logger.GetLogger().WithField("error", err.Error()).Error("Failed to subscribe to WebSocket, falling back to REST")
		h.wsClient.Close()
		h.setFallbackMode(true)
		return nil
	}

	h.setFallbackMode(false)
	logger.GetLogger().Info("WebSocket connection established and subscribed to ticker data")

	return nil
}

// GetTickerData retrieves ticker data using WebSocket cache or REST fallback
func (h *HybridClient) GetTickerData(pairs []string) (*model.KrakenResponse, error) {
	start := time.Now()

	// Try WebSocket first if available and not in fallback mode
	if h.shouldUseWebSocket() {
		wsData, err := h.getTickerFromWebSocket(pairs)
		if err == nil && wsData != nil {
			duration := time.Since(start)
			metrics.RecordKrakenRequest(200, duration) // WebSocket success
			return wsData, nil
		}

		logger.GetLogger().WithField("error", err.Error()).Warn("WebSocket data unavailable, falling back to REST")
		h.setFallbackMode(true)
	}

	// Fallback to REST API
	logger.GetLogger().Debug("Using REST API for ticker data")
	return h.restClient.GetTickerData(pairs)
}

// GetTickerDataWithContext retrieves ticker data with context
func (h *HybridClient) GetTickerDataWithContext(ctx context.Context, pairs []string) (*model.KrakenResponse, error) {
	start := time.Now()

	// Try WebSocket first if available and not in fallback mode
	if h.shouldUseWebSocket() {
		wsData, err := h.getTickerFromWebSocket(pairs)
		if err == nil && wsData != nil {
			duration := time.Since(start)
			metrics.RecordKrakenRequest(200, duration) // WebSocket success
			return wsData, nil
		}

		logger.GetLogger().WithField("error", err.Error()).Warn("WebSocket data unavailable, falling back to REST")
		h.setFallbackMode(true)
	}

	// Fallback to REST API with context
	return h.restClient.GetTickerDataWithContext(ctx, pairs)
}

// shouldUseWebSocket determines if WebSocket should be used
func (h *HybridClient) shouldUseWebSocket() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if !h.wsEnabled || h.wsClient == nil {
		return false
	}

	if h.fallbackMode {
		// Check if we should try WebSocket again
		if h.wsClient.IsConnected() && time.Since(h.lastWSUpdate) < 2*time.Minute {
			h.fallbackMode = false // Try WebSocket again
			return true
		}
		return false
	}

	return h.wsClient.IsConnected()
}

// getTickerFromWebSocket retrieves ticker data from WebSocket cache
func (h *HybridClient) getTickerFromWebSocket(pairs []string) (*model.KrakenResponse, error) {
	if h.wsClient == nil {
		return nil, fmt.Errorf("WebSocket client not available")
	}

	if !h.wsClient.IsConnected() {
		return nil, fmt.Errorf("WebSocket not connected")
	}

	latestPrices := h.wsClient.GetLatestPrices()

	if len(latestPrices) == 0 {
		return nil, fmt.Errorf("no WebSocket price data available")
	}

	// Check if we have fresh data (within last 2 minutes)
	lastUpdate := h.wsClient.GetLastUpdateTime()
	if time.Since(lastUpdate) > 2*time.Minute {
		return nil, fmt.Errorf("WebSocket data is stale")
	}

	// Build response in Kraken format
	result := make(map[string]model.KrakenTickerData)

	for _, pair := range pairs {
		if price, exists := latestPrices[pair]; exists {
			// Convert to Kraken pair format
			krakenPair, krakenExists := model.SupportedPairs[pair]
			if !krakenExists {
				continue
			}

			result[krakenPair] = model.KrakenTickerData{
				LastTradeClosed: []string{fmt.Sprintf("%.8f", price), "0"},
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no matching pairs found in WebSocket data")
	}

	return &model.KrakenResponse{
		Error:  []string{},
		Result: result,
	}, nil
}

// handleWebSocketPriceUpdate handles price updates from WebSocket
func (h *HybridClient) handleWebSocketPriceUpdate(pair string, price float64) {
	h.mu.Lock()
	h.lastWSUpdate = time.Now()
	if h.fallbackMode {
		logger.GetLogger().Info("WebSocket data received, switching back from REST fallback")
		h.fallbackMode = false
	}
	h.mu.Unlock()

	logger.GetLogger().WithFields(map[string]interface{}{
		"pair":  pair,
		"price": price,
	}).Debug("WebSocket price update received")
}

// setFallbackMode sets the fallback mode status
func (h *HybridClient) setFallbackMode(fallback bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fallbackMode = fallback
}

// IsFallbackMode returns whether the client is in fallback mode
func (h *HybridClient) IsFallbackMode() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.fallbackMode
}

// IsWebSocketConnected returns WebSocket connection status
func (h *HybridClient) IsWebSocketConnected() bool {
	if h.wsClient == nil {
		return false
	}
	return h.wsClient.IsConnected()
}

// GetConnectionStatus returns detailed connection status
func (h *HybridClient) GetConnectionStatus() map[string]interface{} {
	status := map[string]interface{}{
		"websocket_enabled":   h.wsEnabled,
		"websocket_connected": h.IsWebSocketConnected(),
		"fallback_mode":       h.IsFallbackMode(),
		"rest_available":      true, // REST is always available
	}

	if h.wsClient != nil {
		status["last_ws_update"] = h.wsClient.GetLastUpdateTime()
	}

	return status
}

// Close closes all connections
func (h *HybridClient) Close() error {
	if h.wsClient != nil {
		return h.wsClient.Close()
	}
	return nil
}

// Health check for monitoring
func (h *HybridClient) HealthCheck() error {
	// Always return healthy if REST is available
	// WebSocket issues should not affect health since we have fallback
	return nil
}

// createRestClientFromConfig crea un cliente REST con la configuración de rate limiting apropiada
func createRestClientFromConfig(cfg config.KrakenConfig) *Client {
	// Convertir config.RateLimitConfig a ratelimit.RateLimitConfig
	rateLimitConfig := ratelimit.RateLimitConfig{
		Enabled:      cfg.RateLimit.Enabled,
		Conservative: cfg.RateLimit.Conservative,
		Capacity:     cfg.RateLimit.Capacity,
		RefillRate:   cfg.RateLimit.RefillRate,
		RefillPeriod: cfg.RateLimit.RefillPeriod,
	}

	// Crear rate limiter desde configuración
	rateLimiter := ratelimit.NewKrakenRateLimiterFromConfig(rateLimitConfig)

	// Crear cliente REST con rate limiter personalizado
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:     KrakenAPIBaseURL,
		timeout:     cfg.Timeout,
		rateLimiter: rateLimiter,
	}
}

// GetRateLimitStats retorna estadísticas del rate limiter del cliente REST
func (h *HybridClient) GetRateLimitStats() map[string]interface{} {
	if h.restClient == nil {
		return map[string]interface{}{
			"error": "REST client not available",
		}
	}
	return h.restClient.GetRateLimitStats()
}

// EnableRateLimit habilita o deshabilita el rate limiting en el cliente REST
func (h *HybridClient) EnableRateLimit(enabled bool) {
	if h.restClient != nil {
		h.restClient.EnableRateLimit(enabled)
	}
}

// GetPairMapper returns the PairMapper instance
func (h *HybridClient) GetPairMapper() *pairs.PairMapper {
	return h.pairMapper
}
