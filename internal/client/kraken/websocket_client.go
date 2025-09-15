package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"btc-ltp-service/internal/logger"
	"btc-ltp-service/internal/metrics"
	"btc-ltp-service/internal/model"
	"btc-ltp-service/internal/pairs"

	"github.com/gorilla/websocket"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WebSocketClient manages WebSocket connection to Kraken
type WebSocketClient struct {
	conn               *websocket.Conn
	url                string
	subscriptions      []string
	priceUpdates       map[string]float64
	mu                 sync.RWMutex
	isConnected        bool
	reconnectTries     int
	maxReconnectTries  int
	reconnectDelay     time.Duration
	ctx                context.Context
	cancel             context.CancelFunc
	onPriceUpdate      func(pair string, price float64)
	timeout            time.Duration
	lastPriceUpdate    time.Time
	channelToSymbolMap map[int]string
	pingInterval       time.Duration
	pongTimeout        time.Duration
	lastPong           time.Time
	pairMapper         *pairs.PairMapper
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(wsURL string, timeout time.Duration, maxReconnectTries int, reconnectDelay time.Duration, pairMapper *pairs.PairMapper) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClient{
		url:                wsURL,
		priceUpdates:       make(map[string]float64),
		maxReconnectTries:  maxReconnectTries,
		reconnectDelay:     reconnectDelay,
		ctx:                ctx,
		cancel:             cancel,
		timeout:            timeout,
		channelToSymbolMap: make(map[int]string),
		pingInterval:       45 * time.Second, // Send ping every 45 seconds (less frequent)
		pongTimeout:        15 * time.Second, // Wait 15 seconds for pong response
		lastPong:           time.Now(),
		pairMapper:         pairMapper,
	}
}

// Connect establishes WebSocket connection and starts message handling
func (w *WebSocketClient) Connect() error {
	logger.GetLogger().Info("Attempting to connect to Kraken WebSocket")

	u, err := url.Parse(w.url)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = w.timeout

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Set connection timeouts - more generous for network issues
	conn.SetReadDeadline(time.Now().Add(w.timeout + 30*time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// Set pong handler
	conn.SetPongHandler(func(data string) error {
		w.mu.Lock()
		w.lastPong = time.Now()
		w.mu.Unlock()
		logger.GetLogger().Debug("WebSocket pong received")
		return nil
	})

	w.mu.Lock()
	w.conn = conn
	w.isConnected = true
	w.reconnectTries = 0
	w.lastPong = time.Now()
	w.mu.Unlock()

	logger.GetLogger().Info("WebSocket connection established")

	// Start message handling goroutine
	go w.handleMessages()

	// Start ping routine to keep connection alive
	go w.startPingRoutine()

	return nil
}

// startPingRoutine sends periodic pings to keep the connection alive
func (w *WebSocketClient) startPingRoutine() {
	ticker := time.NewTicker(w.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.mu.RLock()
			connected := w.isConnected
			conn := w.conn
			lastPong := w.lastPong
			w.mu.RUnlock()

			if !connected || conn == nil {
				return
			}

			// Check if we've received a pong recently
			if time.Since(lastPong) > w.pingInterval+w.pongTimeout {
				logger.GetLogger().Warn("WebSocket ping timeout - connection may be dead")
				conn.Close() // This will trigger handleMessages to exit and reconnect
				return
			}

			// Send ping with more generous write timeout
			conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				logger.GetLogger().WithFields(map[string]interface{}{
					"error": err.Error(),
					"type":  "ping_write_failed",
				}).Warn("Failed to send WebSocket ping - will trigger reconnection")
				conn.Close() // Force reconnection
				return
			}

			logger.GetLogger().Debug("WebSocket ping sent")
		}
	}
}

// Subscribe subscribes to ticker data for the specified pairs
func (w *WebSocketClient) Subscribe(pairs []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isConnected || w.conn == nil {
		return fmt.Errorf("WebSocket not connected")
	}

	// Check if PairMapper is initialized
	if w.pairMapper == nil || !w.pairMapper.IsInitialized() {
		// Fallback to old method for backward compatibility
		logger.GetLogger().Warn("PairMapper not available, using legacy pair mappings")
		krakenPairs := make([]string, 0, len(pairs))
		for _, pair := range pairs {
			if krakenPair, exists := model.SupportedPairs[pair]; exists {
				krakenPairs = append(krakenPairs, krakenPair)
			} else {
				return fmt.Errorf("unsupported pair: %s", pair)
			}
		}

		subscription := map[string]interface{}{
			"event": "subscribe",
			"pair":  krakenPairs,
			"subscription": map[string]interface{}{
				"name": "ticker",
			},
		}

		if err := w.conn.WriteJSON(subscription); err != nil {
			return fmt.Errorf("failed to send subscription message: %w", err)
		}

		w.subscriptions = pairs
		logger.GetLogger().WithField("pairs", pairs).Info("Subscribed to ticker data (legacy mode)")
		return nil
	}

	// Convert standard pair names to Kraken WebSocket format using PairMapper
	wsPairs := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		wsPair, err := w.pairMapper.ToWSFormat(pair)
		if err != nil {
			logger.GetLogger().WithFields(map[string]interface{}{
				"pair":  pair,
				"error": err.Error(),
			}).Error("Failed to convert pair to WebSocket format")
			return fmt.Errorf("unsupported pair: %s - %w", pair, err)
		}
		wsPairs = append(wsPairs, wsPair)
	}

	subscription := map[string]interface{}{
		"event": "subscribe",
		"pair":  wsPairs,
		"subscription": map[string]interface{}{
			"name": "ticker",
		},
	}

	if err := w.conn.WriteJSON(subscription); err != nil {
		return fmt.Errorf("failed to send subscription message: %w", err)
	}

	w.subscriptions = pairs
	logger.GetLogger().WithFields(map[string]interface{}{
		"pairs":    pairs,
		"ws_pairs": wsPairs,
	}).Info("Subscribed to ticker data using PairMapper")

	return nil
}

// handleMessages processes incoming WebSocket messages
func (w *WebSocketClient) handleMessages() {
	defer func() {
		w.mu.Lock()
		w.isConnected = false
		w.mu.Unlock()

		// Attempt reconnection if context is still active
		if w.ctx.Err() == nil {
			w.attemptReconnection()
		}
	}()

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			// Set read deadline with buffer for network delays
			w.conn.SetReadDeadline(time.Now().Add(w.timeout + 30*time.Second))

			messageType, message, err := w.conn.ReadMessage()
			if err != nil {
				// Classify error types for better debugging
				if strings.Contains(err.Error(), "i/o timeout") {
					logger.GetLogger().WithFields(map[string]interface{}{
						"error": err.Error(),
						"type":  "io_timeout",
					}).Warn("WebSocket I/O timeout detected - will reconnect")
					metrics.RecordKrakenError("websocket_io_timeout")
				} else if strings.Contains(err.Error(), "abnormal closure") {
					logger.GetLogger().WithFields(map[string]interface{}{
						"error": err.Error(),
						"type":  "abnormal_closure",
					}).Warn("WebSocket abnormal closure - will reconnect")
					metrics.RecordKrakenError("websocket_abnormal_closure")
				} else {
					logger.GetLogger().WithFields(map[string]interface{}{
						"error": err.Error(),
						"type":  "unknown",
					}).Error("WebSocket read error - will reconnect")
					metrics.RecordKrakenError("websocket_read_error")
				}
				return
			}

			if messageType == websocket.TextMessage {
				w.processMessage(message)
			}
		}
	}
}

// processMessage processes individual WebSocket messages
func (w *WebSocketClient) processMessage(message []byte) {
	// First try to parse as a status message (object)
	var statusMsg model.KrakenWSMessage
	if err := json.Unmarshal(message, &statusMsg); err == nil {
		if statusMsg.Event != "" {
			w.handleStatusMessage(statusMsg)
			return
		}
	}

	// Try to parse as array message (ticker data)
	var tickerArray []interface{}
	if err := json.Unmarshal(message, &tickerArray); err != nil {
		// Try to parse as object message (subscription confirmations, etc.)
		var objMsg map[string]interface{}
		if err2 := json.Unmarshal(message, &objMsg); err2 == nil {
			// Handle object-type messages
			if event, exists := objMsg["event"]; exists {
				logger.GetLogger().WithFields(map[string]interface{}{
					"event":   event,
					"message": objMsg,
				}).Debug("WebSocket object message received")
				return
			}
		}

		logger.GetLogger().WithFields(map[string]interface{}{
			"error":   err.Error(),
			"message": string(message)[:min(100, len(message))], // First 100 chars for debugging
		}).Debug("Unable to parse WebSocket message")
		return
	}

	if len(tickerArray) < 4 {
		return // Not a ticker message
	}

	// Extract channel ID and channel name
	channelIDFloat, ok := tickerArray[0].(float64)
	if !ok {
		return
	}
	channelID := int(channelIDFloat)

	channelName, ok := tickerArray[2].(string)
	if !ok || channelName != "ticker" {
		return
	}

	// Extract pair name
	pairName, ok := tickerArray[3].(string)
	if !ok {
		return
	}

	// Store channel to symbol mapping
	w.mu.Lock()
	w.channelToSymbolMap[channelID] = pairName
	w.mu.Unlock()

	// Parse ticker data
	tickerDataRaw, ok := tickerArray[1].(map[string]interface{})
	if !ok {
		return
	}

	w.handleTickerData(pairName, tickerDataRaw)
}

// handleStatusMessage processes status messages (subscription confirmations, errors, etc.)
func (w *WebSocketClient) handleStatusMessage(msg model.KrakenWSMessage) {
	switch msg.Event {
	case "systemStatus":
		logger.GetLogger().WithField("status", msg.Data).Info("WebSocket system status")
	case "subscriptionStatus":
		logger.GetLogger().WithFields(map[string]interface{}{
			"status":       "subscription_confirmed",
			"subscription": msg,
		}).Info("WebSocket subscription confirmed")
	case "heartbeat":
		// Update last activity timestamp
		w.mu.Lock()
		w.lastPriceUpdate = time.Now()
		w.mu.Unlock()
		logger.GetLogger().Debug("WebSocket heartbeat received")
	default:
		if msg.ErrorMessage != "" {
			logger.GetLogger().WithField("error", msg.ErrorMessage).Error("WebSocket error")
			metrics.RecordKrakenError("websocket_api_error")
		} else if msg.Event != "" {
			logger.GetLogger().WithField("event", msg.Event).Debug("WebSocket event received")
		}
	}
}

// handleTickerData processes ticker data and extracts the last traded price
func (w *WebSocketClient) handleTickerData(krakenPair string, tickerData map[string]interface{}) {
	var standardPair string
	var err error

	// Try to convert using PairMapper first
	if w.pairMapper != nil && w.pairMapper.IsInitialized() {
		standardPair, err = w.pairMapper.ToStandardFromWS(krakenPair)
		if err != nil {
			logger.GetLogger().WithFields(map[string]interface{}{
				"pair":  krakenPair,
				"error": err.Error(),
			}).Warn("Failed to convert WebSocket pair using PairMapper, trying legacy mapping")

			// Fallback to legacy mapping
			if legacyPair, exists := model.KrakenToStandardPair[krakenPair]; exists {
				standardPair = legacyPair
			} else {
				logger.GetLogger().WithField("pair", krakenPair).Warn("Unknown Kraken pair received (not in PairMapper or legacy mapping)")
				return
			}
		}
	} else {
		// Use legacy mapping
		if legacyPair, exists := model.KrakenToStandardPair[krakenPair]; exists {
			standardPair = legacyPair
		} else {
			logger.GetLogger().WithField("pair", krakenPair).Warn("Unknown Kraken pair received (PairMapper not available)")
			return
		}
	}

	// Extract last trade closed price
	closeData, ok := tickerData["c"].([]interface{})
	if !ok || len(closeData) == 0 {
		return
	}

	priceStr, ok := closeData[0].(string)
	if !ok {
		return
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		logger.GetLogger().WithFields(map[string]interface{}{
			"pair":      standardPair,
			"price_str": priceStr,
			"error":     err.Error(),
		}).Error("Failed to parse price from WebSocket")
		return
	}

	// Update internal price store
	w.mu.Lock()
	w.priceUpdates[standardPair] = price
	w.lastPriceUpdate = time.Now()
	w.mu.Unlock()

	// Call price update callback if set
	if w.onPriceUpdate != nil {
		w.onPriceUpdate(standardPair, price)
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"pair":    standardPair,
		"ws_pair": krakenPair,
		"price":   price,
	}).Debug("WebSocket price update")

	// Update metrics
	metrics.UpdateCurrentPrice(standardPair, price)
}

// attemptReconnection tries to reconnect to WebSocket with exponential backoff
func (w *WebSocketClient) attemptReconnection() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			w.mu.Lock()
			if w.reconnectTries >= w.maxReconnectTries {
				logger.GetLogger().Error("Max WebSocket reconnection attempts reached")
				w.mu.Unlock()
				return
			}

			w.reconnectTries++
			currentTry := w.reconnectTries
			w.mu.Unlock()

			// Exponential backoff: 5s, 10s, 20s, 40s, 80s
			backoffDelay := w.reconnectDelay * time.Duration(1<<(currentTry-1))
			if backoffDelay > 2*time.Minute {
				backoffDelay = 2 * time.Minute // Cap at 2 minutes
			}

			logger.GetLogger().WithFields(map[string]interface{}{
				"attempt": currentTry,
				"delay":   backoffDelay.String(),
			}).Info("Attempting WebSocket reconnection")

			time.Sleep(backoffDelay)

			if err := w.Connect(); err != nil {
				logger.GetLogger().WithField("error", err.Error()).Error("WebSocket reconnection failed")
				metrics.RecordKrakenRetry()
				continue
			}

			// Re-subscribe to ticker data
			if len(w.subscriptions) > 0 {
				if err := w.Subscribe(w.subscriptions); err != nil {
					logger.GetLogger().WithField("error", err.Error()).Error("Failed to re-subscribe after reconnection")
					w.Close()
					continue
				}
			}

			logger.GetLogger().Info("WebSocket reconnection successful")
			return
		}
	}
}

// GetLatestPrices returns the latest prices for all subscribed pairs
func (w *WebSocketClient) GetLatestPrices() map[string]float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	prices := make(map[string]float64)
	for pair, price := range w.priceUpdates {
		prices[pair] = price
	}

	return prices
}

// IsConnected returns the current connection status
func (w *WebSocketClient) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isConnected
}

// SetPriceUpdateCallback sets a callback function for price updates
func (w *WebSocketClient) SetPriceUpdateCallback(callback func(pair string, price float64)) {
	w.onPriceUpdate = callback
}

// GetLastUpdateTime returns the timestamp of the last price update
func (w *WebSocketClient) GetLastUpdateTime() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastPriceUpdate
}

// Close closes the WebSocket connection
func (w *WebSocketClient) Close() error {
	logger.GetLogger().Info("Closing WebSocket connection")

	w.cancel() // Cancel context to stop reconnection attempts

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		w.isConnected = false
		return err
	}

	return nil
}
