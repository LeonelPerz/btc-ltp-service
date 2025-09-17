package kraken

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/config"
	cachepkg "btc-ltp-service/internal/infrastructure/repositories/cache"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Thread-safe WebSocket connection wrapper
type safeWebSocketConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *safeWebSocketConn) WriteJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

func (s *safeWebSocketConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

// Mock WebSocket Server
type mockWebSocketServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	messages chan []byte
	clients  []*safeWebSocketConn
	mu       sync.Mutex
}

func newMockWebSocketServer() *mockWebSocketServer {
	mws := &mockWebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		messages: make(chan []byte, 100),
		clients:  make([]*safeWebSocketConn, 0),
	}

	mws.server = httptest.NewServer(http.HandlerFunc(mws.handleWebSocket))
	return mws
}

func (mws *mockWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := mws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	safeConn := &safeWebSocketConn{conn: conn}

	mws.mu.Lock()
	mws.clients = append(mws.clients, safeConn)
	mws.mu.Unlock()

	// Send subscription confirmation
	subscriptionConfirm := WebSocketMessage{
		Event:  "subscriptionStatus",
		Status: "subscribed",
		Pair:   []string{"XBT/USD"},
		Subscription: map[string]interface{}{
			"name": "ticker",
		},
	}

	_ = safeConn.WriteJSON(subscriptionConfirm)

	// Listen for messages and echo them back
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		mws.messages <- message
	}
}

func (mws *mockWebSocketServer) sendTickerUpdate(pair string, price string) {
	tickerUpdate := []interface{}{
		1, // channel ID
		map[string]interface{}{
			"c": []interface{}{price, "1.0"},        // last trade closed
			"a": []interface{}{"50001.0", "1", "1"}, // ask
			"b": []interface{}{"49999.0", "1", "1"}, // bid
		},
		"ticker",
		pair,
	}

	mws.mu.Lock()
	clients := make([]*safeWebSocketConn, len(mws.clients))
	copy(clients, mws.clients)
	mws.mu.Unlock()

	for _, client := range clients {
		_ = client.WriteJSON(tickerUpdate)
	}
}

func (mws *mockWebSocketServer) close() {
	mws.mu.Lock()
	clients := make([]*safeWebSocketConn, len(mws.clients))
	copy(clients, mws.clients)
	mws.clients = mws.clients[:0]
	mws.mu.Unlock()

	for _, client := range clients {
		_ = client.Close()
	}
	mws.server.Close()
}

func (mws *mockWebSocketServer) getURL() string {
	return "ws" + strings.TrimPrefix(mws.server.URL, "http")
}

// Test Helper Functions for WebSocket
func createTestWebSocketClient(url string) *WebSocketClient {
	cache := cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), 60*time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketClient{
		url:           url,
		subscriptions: make(map[string]bool),
		priceChannels: make(map[string]chan *entities.Price),
		cache:         cache,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// ===== CASOS DE ÉXITO - FUNCIONAMIENTO NORMAL =====

func TestNewWebSocketClient_DefaultConfiguration(t *testing.T) {
	client := NewWebSocketClient()

	assert.NotNil(t, client)
	assert.Equal(t, KrakenWebSocketURL, client.url)
	assert.NotNil(t, client.subscriptions)
	assert.NotNil(t, client.priceChannels)
	assert.NotNil(t, client.cache)
}

func TestNewWebSocketClientWithConfig_CustomConfiguration(t *testing.T) {
	cfg := config.KrakenConfig{
		WebSocketURL: "wss://custom-ws.kraken.com",
	}

	client := NewWebSocketClientWithConfig(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg.WebSocketURL, client.url)
	assert.NotNil(t, client.cache)
}

func TestWebSocketClient_Connect_Success(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()

	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Cleanup
	_ = client.Close()
}

func TestWebSocketClient_Close_Success(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
	assert.False(t, client.IsConnected())
}

func TestWebSocketClient_SubscribeTicker_Success(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	assert.NoError(t, err)

	// Verificar que la suscripción se registró
	assert.True(t, client.subscriptions["BTC/USD"])
	assert.NotNil(t, client.priceChannels["BTC/USD"])
}

func TestWebSocketClient_GetTicker_Success(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	require.NoError(t, err)

	// Simular recepción de datos
	go func() {
		time.Sleep(100 * time.Millisecond)
		mockServer.sendTickerUpdate("XBT/USD", "50000.0")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	price, err := client.GetTicker(ctx, "BTC/USD")
	require.NoError(t, err)
	require.NotNil(t, price)
	assert.Equal(t, "BTC/USD", price.Pair)
	assert.Equal(t, 50000.0, price.Amount)
}

func TestWebSocketClient_GetTickers_Success(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	pairs := []string{"BTC/USD", "ETH/USD"}
	err = client.SubscribeTicker(pairs)
	require.NoError(t, err)

	// Simular recepción de datos para ambos pares
	go func() {
		time.Sleep(100 * time.Millisecond)
		mockServer.sendTickerUpdate("XBT/USD", "50000.0")
		time.Sleep(50 * time.Millisecond)
		mockServer.sendTickerUpdate("ETH/USD", "3000.0")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	prices, err := client.GetTickers(ctx, pairs)
	require.NoError(t, err)
	require.Len(t, prices, 2)

	// Verificar que tenemos ambos pares
	pairMap := make(map[string]*entities.Price)
	for _, price := range prices {
		pairMap[price.Pair] = price
	}
	assert.Contains(t, pairMap, "BTC/USD")
	assert.Contains(t, pairMap, "ETH/USD")
}

// ===== CASOS DE ERROR - MANEJO DE ERRORES Y EXCEPCIONES =====

func TestWebSocketClient_Connect_InvalidURL(t *testing.T) {
	client := createTestWebSocketClient("invalid-url")
	err := client.Connect()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection to kraken failed")
}

func TestWebSocketClient_Connect_AlreadyConnected(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	// Intentar conectar de nuevo
	err = client.Connect()
	assert.NoError(t, err) // No debería dar error, simplemente no hace nada
}

func TestWebSocketClient_SubscribeTicker_NotConnected(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	err := client.SubscribeTicker([]string{"BTC/USD"})
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionFailed, err)
}

func TestWebSocketClient_GetTicker_NotConnected(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	ctx := context.Background()
	_, err := client.GetTicker(ctx, "BTC/USD")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection to kraken failed")
}

func TestWebSocketClient_GetTicker_NotSubscribed(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = client.GetTicker(ctx, "BTC/USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled/timeout")
}

func TestWebSocketClient_GetTicker_InvalidPair(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"INVALID"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert pair to Kraken WS format")
}

// ===== EDGE CASES - CASOS LÍMITE Y SITUACIONES EXTREMAS =====

func TestWebSocketClient_SubscribeTicker_EmptyPairs(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{})
	assert.NoError(t, err) // Debería manejar lista vacía sin error
}

func TestWebSocketClient_Close_NotConnected(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	err := client.Close()
	assert.NoError(t, err) // No debería dar error cerrar una conexión no establecida
}

func TestWebSocketClient_GetTicker_Timeout(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.GetTicker(ctx, "BTC/USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// ===== CONCURRENCIA - ACCESO CONCURRENTE Y THREAD-SAFETY =====

func TestWebSocketClient_ConcurrentSubscriptions(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			pair := fmt.Sprintf("PAIR%d/USD", index)
			if err := client.SubscribeTicker([]string{pair}); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Verificar que no hubo errores de concurrencia
	for err := range errors {
		if err != nil && !strings.Contains(err.Error(), "failed to convert pair") {
			t.Errorf("Unexpected error in concurrent subscription: %v", err)
		}
	}
}

func TestWebSocketClient_ConcurrentGetTicker(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	require.NoError(t, err)

	// Enviar datos continuamente
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; i < 50; i++ {
			<-ticker.C
			mockServer.sendTickerUpdate("XBT/USD", fmt.Sprintf("5000%d.0", i))
		}
	}()

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *entities.Price, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

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
		t.Errorf("Unexpected error in concurrent GetTicker: %v", err)
	}

	// Verificar que todas las respuestas son válidas
	count := 0
	for price := range results {
		assert.Equal(t, "BTC/USD", price.Pair)
		assert.Greater(t, price.Amount, 0.0)
		count++
	}
	assert.Equal(t, numGoroutines, count)
}

// ===== TIMEOUTS Y CANCELACIONES - MANEJO DE CONTEXTOS =====

func TestWebSocketClient_GetTicker_ContextCanceled(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	_, err = client.GetTicker(ctx, "BTC/USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWebSocketClient_GetTickers_ContextTimeout(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD", "ETH/USD"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.GetTickers(ctx, []string{"BTC/USD", "ETH/USD"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// ===== RECONEXIÓN - LÓGICA DE RECONEXIÓN AUTOMÁTICA =====

func TestWebSocketClient_GetReconnectionStatus_NotReconnecting(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	isReconnecting, attemptCount := client.GetReconnectionStatus()
	assert.False(t, isReconnecting)
	assert.Equal(t, 0, attemptCount)
}

func TestWebSocketClient_IsConnected_States(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())

	// Initially not connected
	assert.False(t, client.IsConnected())

	// Connect
	err := client.Connect()
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Close
	err = client.Close()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// ===== CACHE - INTEGRACIÓN CON SISTEMA DE CACHE =====

func TestWebSocketClient_GetPriceCache(t *testing.T) {
	client := NewWebSocketClient()

	retrievedCache := client.GetPriceCache()
	assert.NotNil(t, retrievedCache)
}

func TestWebSocketClient_CacheIntegration_UpdateOnTicker(t *testing.T) {
	mockServer := newMockWebSocketServer()
	defer mockServer.close()

	client := createTestWebSocketClient(mockServer.getURL())
	err := client.Connect()
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	err = client.SubscribeTicker([]string{"BTC/USD"})
	require.NoError(t, err)

	// Enviar actualización de ticker
	mockServer.sendTickerUpdate("XBT/USD", "50000.0")

	// Esperar un poco para que se procese
	time.Sleep(100 * time.Millisecond)

	// Verificar que se actualizó el cache
	ctx := context.Background()
	cachedPrice, found := client.cache.Get(ctx, "BTC/USD")
	assert.True(t, found)
	assert.Equal(t, "BTC/USD", cachedPrice.Pair)
	assert.Equal(t, 50000.0, cachedPrice.Amount)
}

// ===== PAIR CONVERSION TESTS =====

func TestToWebSocketPair_ValidPairs(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"BTC/USD", "XBT/USD"},
		{"ETH/USD", "ETH/USD"},
		{"BTC/EUR", "XBT/EUR"},
		{"ETH/EUR", "ETH/EUR"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := toWebSocketPair(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToWebSocketPair_InvalidPairs(t *testing.T) {
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
			_, err := toWebSocketPair(tc)
			assert.Error(t, err)
		})
	}
}

func TestFromWebSocketPair_ValidPairs(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"XBT/USD", "BTC/USD"},
		{"ETH/USD", "ETH/USD"},
		{"XBT/EUR", "BTC/EUR"},
		{"ETH/EUR", "ETH/EUR"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := fromWebSocketPair(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFromWebSocketPair_InvalidPairs(t *testing.T) {
	testCases := []string{
		"INVALID",
		"",
		"XBT",
		"USD",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := fromWebSocketPair(tc)
			assert.Error(t, err)
		})
	}
}

// ===== MESSAGE HANDLING TESTS =====

func TestWebSocketClient_handleTickerUpdate_ValidData(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	// Preparar datos de ticker válidos
	tickerData := []interface{}{
		1, // channel ID
		map[string]interface{}{
			"c": []interface{}{"50000.0", "1.0"}, // last trade closed
		},
		"ticker",
		"XBT/USD",
	}

	// Agregar suscripción y canal manualmente para el test
	client.subscriptions["BTC/USD"] = true
	client.priceChannels["BTC/USD"] = make(chan *entities.Price, 1)

	err := client.handleTickerUpdate(tickerData)
	assert.NoError(t, err)

	// Verificar que se envió un precio al canal
	select {
	case price := <-client.priceChannels["BTC/USD"]:
		assert.Equal(t, "BTC/USD", price.Pair)
		assert.Equal(t, 50000.0, price.Amount)
	case <-time.After(100 * time.Millisecond):
		t.Error("No se recibió precio en el canal")
	}
}

func TestWebSocketClient_handleTickerUpdate_InvalidFormat(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	// Datos con formato inválido (muy pocos elementos)
	tickerData := []interface{}{1, 2} // Solo 2 elementos, necesita al menos 4

	err := client.handleTickerUpdate(tickerData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ticker update format")
}

func TestWebSocketClient_handleEventMessage_SubscriptionStatus(t *testing.T) {
	client := createTestWebSocketClient("ws://localhost:9999")

	msg := WebSocketMessage{
		Event:  "subscriptionStatus",
		Status: "subscribed",
		Pair:   []string{"XBT/USD"},
	}

	err := client.handleEventMessage(msg)
	assert.NoError(t, err)
}

// ===== COVERAGE BOOST TESTS =====
// Estos tests ejercitan caminos de error que normalmente no ocurren en producción
// pero ayudan a alcanzar el 80 % de coverage sin afectar la lógica.

func TestHandleTickerUpdate_InvalidCases(t *testing.T) {
	client := &WebSocketClient{}

	// caso 1: array demasiado corto
	if err := client.handleTickerUpdate([]interface{}{1, 2}); err == nil {
		t.Errorf("expected error for short array")
	}

	// caso 2: tickerData no es map
	if err := client.handleTickerUpdate([]interface{}{1, "not-a-map", "ticker", "XBT/USD"}); err == nil {
		t.Errorf("expected error for invalid tickerData")
	}

	// caso 3: pair no es string
	if err := client.handleTickerUpdate([]interface{}{1, map[string]interface{}{}, "ticker", 123}); err == nil {
		t.Errorf("expected error for invalid pair")
	}
}

func TestHandleMessage_UnrecognisedPayload(t *testing.T) {
	client := &WebSocketClient{}
	raw := []byte("unrecognised") // no es JSON válido ni array
	if err := client.handleMessage(raw); err != nil {
		t.Errorf("handleMessage should ignore unknown payloads, got %v", err)
	}
}

func TestWebSocketClient_findOriginalPairFromKraken(t *testing.T) {
	tests := []struct {
		name           string
		krakenPair     string
		subscriptions  map[string]bool
		expectedResult string
	}{
		{
			name:       "find existing pair in subscriptions - BTC/USD",
			krakenPair: "XXBTZUSD",
			subscriptions: map[string]bool{
				"BTC/USD": true,
				"ETH/USD": true,
			},
			expectedResult: "BTC/USD",
		},
		{
			name:       "find existing pair in subscriptions - ETH/USD",
			krakenPair: "XETHZUSD",
			subscriptions: map[string]bool{
				"BTC/USD": true,
				"ETH/USD": true,
			},
			expectedResult: "ETH/USD",
		},
		{
			name:       "pair not found in subscriptions but valid kraken pair",
			krakenPair: "XXBTZUSD",
			subscriptions: map[string]bool{
				"ETH/USD": true,
			},
			expectedResult: "BTC/USD", // Should use FromKrakenPair fallback
		},
		{
			name:       "invalid kraken pair",
			krakenPair: "INVALID",
			subscriptions: map[string]bool{
				"BTC/USD": true,
			},
			expectedResult: "", // Should return empty string
		},
		{
			name:           "empty subscriptions with valid kraken pair",
			krakenPair:     "XXBTZUSD",
			subscriptions:  map[string]bool{},
			expectedResult: "BTC/USD", // Should use FromKrakenPair fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &WebSocketClient{
				subscriptions: tt.subscriptions,
			}

			result := client.findOriginalPairFromKraken(tt.krakenPair)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestWebSocketClient_pingHandler_Integration(t *testing.T) {
	// Test básico para pingHandler - verifica que la función no entre en pánico
	// y termine correctamente cuando el contexto se cancela
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &WebSocketClient{
		ctx: ctx,
	}

	// Crear un WaitGroup para sincronización
	client.wg.Add(1)

	// Ejecutar pingHandler en una goroutine separada
	go client.pingHandler()

	// Cancelar el contexto después de un breve delay para permitir inicialización
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Esperar a que pingHandler termine
	done := make(chan struct{})
	go func() {
		client.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Test pasó - pingHandler terminó correctamente
	case <-time.After(1 * time.Second):
		t.Fatal("pingHandler no terminó en tiempo esperado")
	}
}

func TestWebSocketClient_scheduleReconnect_StateManagement(t *testing.T) {
	tests := []struct {
		name                 string
		initialConnected     bool
		initialReconnecting  bool
		expectedConnected    bool
		expectedReconnecting bool
		shouldSchedule       bool
	}{
		{
			name:                 "schedule reconnect when connected and not reconnecting",
			initialConnected:     true,
			initialReconnecting:  false,
			expectedConnected:    false,
			expectedReconnecting: true,
			shouldSchedule:       true,
		},
		{
			name:                 "do not schedule when already reconnecting",
			initialConnected:     true,
			initialReconnecting:  true,
			expectedConnected:    true,
			expectedReconnecting: true,
			shouldSchedule:       false,
		},
		{
			name:                 "do not schedule when not connected",
			initialConnected:     false,
			initialReconnecting:  false,
			expectedConnected:    false,
			expectedReconnecting: false,
			shouldSchedule:       false,
		},
		{
			name:                 "do not schedule when disconnected and already reconnecting",
			initialConnected:     false,
			initialReconnecting:  true,
			expectedConnected:    false,
			expectedReconnecting: true,
			shouldSchedule:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client := &WebSocketClient{
				ctx:            ctx,
				isConnected:    tt.initialConnected,
				isReconnecting: tt.initialReconnecting,
				reconnectCount: 0,
			}

			// Llamar scheduleReconnect
			client.scheduleReconnect()

			// Verificar estado final
			client.mu.RLock()
			actualConnected := client.isConnected
			actualReconnecting := client.isReconnecting
			hasTimer := client.reconnectTimer != nil
			client.mu.RUnlock()

			assert.Equal(t, tt.expectedConnected, actualConnected, "Estado de conexión incorrecto")
			assert.Equal(t, tt.expectedReconnecting, actualReconnecting, "Estado de reconexión incorrecto")

			if tt.shouldSchedule {
				assert.NotNil(t, hasTimer, "Debería haber programado un timer de reconexión")

				// Cleanup: detener el timer si existe
				client.mu.Lock()
				if client.reconnectTimer != nil {
					client.reconnectTimer.Stop()
					client.reconnectTimer = nil
				}
				client.mu.Unlock()
			} else {
				assert.False(t, hasTimer, "No debería haber programado un timer de reconexión")
			}
		})
	}
}
