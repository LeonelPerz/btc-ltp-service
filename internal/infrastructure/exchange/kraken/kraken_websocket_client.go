package kraken

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/metrics"
	cachepkg "btc-ltp-service/internal/infrastructure/repositories/cache"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	KrakenWebSocketURL = "wss://ws.kraken.com"
	PingInterval       = 30 * time.Second
	WriteWait          = 10 * time.Second
	PongWait           = 60 * time.Second
	ReadBufferSize     = 1024
	WriteBufferSize    = 1024
)

// WebSocketClient implementa la interfaz Exchange usando WebSocket de Kraken
type WebSocketClient struct {
	conn           *websocket.Conn
	url            string
	mu             sync.RWMutex
	isConnected    bool
	subscriptions  map[string]bool // pairs suscritos
	priceChannels  map[string]chan *entities.Price
	cache          *cachepkg.PriceCacheAdapter
	ctx            context.Context
	cancel         context.CancelFunc
	reconnectTimer *time.Timer
	isReconnecting bool
	reconnectCount int
	wg             sync.WaitGroup // espera a que goroutines terminen al cerrar
}

// WebSocketMessage representa un mensaje general de WebSocket de Kraken
type WebSocketMessage struct {
	Event        string      `json:"event,omitempty"`
	Pair         []string    `json:"pair,omitempty"`
	Subscription interface{} `json:"subscription,omitempty"`
	ReqID        int         `json:"reqid,omitempty"`
	Status       string      `json:"status,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
}

// TickerSubscription representa la suscripción al canal de ticker
type TickerSubscription struct {
	Name string `json:"name"`
}

// TickerUpdate representa una actualización de ticker de Kraken via WebSocket
type TickerUpdate struct {
	ChannelID   int                    `json:"-"`
	Data        map[string]interface{} `json:"-"`
	ChannelName string                 `json:"-"`
	Pair        string                 `json:"-"`
}

// NewWebSocketClient crea una nueva instancia del cliente WebSocket de Kraken
func NewWebSocketClient() *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketClient{
		url:           KrakenWebSocketURL,
		subscriptions: make(map[string]bool),
		priceChannels: make(map[string]chan *entities.Price),
		cache:         cachepkg.NewPriceCache(cachepkg.NewMemoryCache(), 30*time.Second),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// NewWebSocketClientWithConfig crea una nueva instancia del cliente WebSocket de Kraken con configuración
func NewWebSocketClientWithConfig(cfg config.KrakenConfig) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())
	ttl := cfg.PriceCacheTTL
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	backend := cachepkg.NewMemoryCache()
	return &WebSocketClient{
		url:           cfg.WebSocketURL,
		subscriptions: make(map[string]bool),
		priceChannels: make(map[string]chan *entities.Price),
		cache:         cachepkg.NewPriceCache(backend, ttl),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// --- Helpers de mapeo específicos para WebSocket ---
// Kraken WebSocket usa nombres "amistosos" (XBT/USD), no los códigos internos (XXBTZUSD)
var wsAssetMap = map[string]string{
	"BTC": "XBT",
	"ETH": "ETH",
	"LTC": "LTC",
	"XRP": "XRP",
	"USD": "USD",
	"EUR": "EUR",
	"CHF": "CHF",
	"JPY": "JPY",
	"GBP": "GBP",
	"CAD": "CAD",
}

func toWebSocketPair(symbol string) (string, error) {
	s := strings.ToUpper(symbol)
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid pair format, expected BASE/QUOTE: %s", symbol)
	}
	base, ok := wsAssetMap[parts[0]]
	if !ok {
		return "", fmt.Errorf("unsupported base asset for WS: %s", parts[0])
	}
	quote, ok := wsAssetMap[parts[1]]
	if !ok {
		return "", fmt.Errorf("unsupported quote asset for WS: %s", parts[1])
	}
	return base + "/" + quote, nil
}

func fromWebSocketPair(wsPair string) (string, error) {
	s := strings.ToUpper(wsPair)
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid WS pair format: %s", wsPair)
	}
	// Construir mapa inverso
	inverse := make(map[string]string, len(wsAssetMap))
	for friendly, wsName := range wsAssetMap {
		inverse[wsName] = friendly
	}
	base, ok := inverse[parts[0]]
	if !ok {
		return "", fmt.Errorf("unsupported WS base asset: %s", parts[0])
	}
	quote, ok := inverse[parts[1]]
	if !ok {
		return "", fmt.Errorf("unsupported WS quote asset: %s", parts[1])
	}
	return base + "/" + quote, nil
}

// Connect establece la conexión WebSocket con Kraken
func (k *WebSocketClient) Connect() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.isConnected {
		return nil
	}

	u, err := url.Parse(k.url)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	dialer := websocket.Dialer{
		ReadBufferSize:  ReadBufferSize,
		WriteBufferSize: WriteBufferSize,
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	k.conn = conn
	k.isConnected = true

	// Configurar timeouts
	_ = k.conn.SetReadDeadline(time.Now().Add(PongWait))
	k.conn.SetPongHandler(func(string) error {
		_ = k.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	// Iniciar goroutines para manejo de mensajes
	k.wg.Add(2)
	go k.readMessages()
	go k.pingHandler()

	// Usar logging estructurado en lugar de log.Println
	// Note: Necesitaríamos el contexto aquí, pero para mantener la interfaz existente,
	// usaremos context.Background() temporalmente
	return nil
}

// Close cierra la conexión WebSocket
func (k *WebSocketClient) Close() error {
	k.mu.Lock()

	if !k.isConnected && !k.isReconnecting {
		k.mu.Unlock()
		return nil
	}

	// Actualizar flags y detener temporizadores bajo el lock
	k.cancel()
	k.isConnected = false
	k.isReconnecting = false
	k.reconnectCount = 0

	if k.reconnectTimer != nil {
		k.reconnectTimer.Stop()
		k.reconnectTimer = nil
	}

	// Capturar conexión actual para cerrarla fuera del lock
	conn := k.conn

	k.mu.Unlock()

	// Cerrar conexión WebSocket para interrumpir ReadMessage
	var err error
	if conn != nil {
		_ = conn.SetReadDeadline(time.Now())
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}

	// Esperar a que goroutines terminen antes de cerrar canales
	k.wg.Wait()

	// Ahora es seguro cerrar canales y limpiar conexión compartida
	k.mu.Lock()
	k.conn = nil
	for _, ch := range k.priceChannels {
		close(ch)
	}
	k.priceChannels = make(map[string]chan *entities.Price)
	k.mu.Unlock()

	return err
}

// SubscribeTicker se suscribe al canal de ticker para los pares especificados
func (k *WebSocketClient) SubscribeTicker(pairs []string) error {
	if !k.isConnected {
		return ErrConnectionFailed
	}

	// Convertir pares a formato WebSocket y crear canales de forma segura
	krakenPairs := make([]string, len(pairs))

	// Proteger acceso al mapa con mutex
	k.mu.Lock()
	for i, pair := range pairs {
		krakenPair, err := toWebSocketPair(pair)
		if err != nil {
			k.mu.Unlock()
			return fmt.Errorf("failed to convert pair to Kraken WS format: %w", err)
		}
		krakenPairs[i] = krakenPair
		k.subscriptions[pair] = true

		// Si el canal ya existe, reutilizarlo para evitar cerrar un canal que
		// podría estar siendo usado por otra goroutine en ese momento.
		if _, exists := k.priceChannels[pair]; !exists {
			k.priceChannels[pair] = make(chan *entities.Price, 100)
		}
	}
	k.mu.Unlock()

	subscribeMsg := WebSocketMessage{
		Event: "subscribe",
		Pair:  krakenPairs,
		Subscription: TickerSubscription{
			Name: "ticker",
		},
		ReqID: int(time.Now().Unix()),
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	_ = k.conn.SetWriteDeadline(time.Now().Add(WriteWait))
	return k.conn.WriteJSON(subscribeMsg)
}

// GetTicker obtiene el último precio usando WebSocket (implementa la interfaz Exchange)
func (k *WebSocketClient) GetTicker(ctx context.Context, pair string) (*entities.Price, error) {
	// 1. Intentar cache
	if k.cache != nil {
		if price, ok := k.cache.Get(ctx, pair); ok {
			return price, nil
		}
	}

	// 2. Si no hay cache, proceder con conexión WS como antes
	if !k.isConnected {
		if err := k.Connect(); err != nil {
			return nil, ErrConnectionFailed
		}
	}

	k.mu.RLock()
	isSubscribed := k.subscriptions[pair]
	k.mu.RUnlock()
	if !isSubscribed {
		if err := k.SubscribeTicker([]string{pair}); err != nil {
			return nil, err
		}
	}

	k.mu.RLock()
	priceChan, exists := k.priceChannels[pair]
	k.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("price channel not found for pair %s", pair)
	}

	select {
	case price := <-priceChan:
		return price, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled/timeout waiting for price update for pair %s: %w", pair, ctx.Err())
	}
}

// GetTickers obtiene precios múltiples usando WebSocket
func (k *WebSocketClient) GetTickers(ctx context.Context, pairs []string) ([]*entities.Price, error) {
	if !k.isConnected {
		// Intentar conexión perezosa
		if err := k.Connect(); err != nil {
			return nil, ErrConnectionFailed
		}
	}

	// 1. Intentar cache primero
	var cached []*entities.Price
	var missing []string
	if k.cache != nil {
		cached, missing = k.cache.GetMany(ctx, pairs)
		if len(missing) == 0 {
			return cached, nil
		}
	} else {
		missing = pairs
	}

	// 2. Suscribirse a pares faltantes
	if err := k.SubscribeTicker(missing); err != nil {
		return nil, err
	}

	// Construir canales sólo para pares faltantes
	k.mu.RLock()
	priceChannels := make([]chan *entities.Price, 0, len(missing))
	for _, pair := range missing {
		if ch, ok := k.priceChannels[pair]; ok {
			priceChannels = append(priceChannels, ch)
		}
	}
	k.mu.RUnlock()

	prices := make([]*entities.Price, 0, len(pairs))
	prices = append(prices, cached...)

	for i := 0; i < len(priceChannels); i++ {
		select {
		case price := <-priceChannels[i]:
			prices = append(prices, price)
			if k.cache != nil {
				_ = k.cache.Set(ctx, price)
			}
		case <-ctx.Done():
			return prices, fmt.Errorf("context canceled/timeout waiting for price updates, got %d out of %d: %w", len(prices), len(pairs), ctx.Err())
		}
	}

	return prices, nil
}

// readMessages lee mensajes del WebSocket en un bucle
func (k *WebSocketClient) readMessages() {
	defer k.wg.Done()

	for {
		select {
		case <-k.ctx.Done():
			return
		default:
			_, messageBytes, err := k.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logging.Error(context.Background(), "WebSocket unexpected close error", logging.Fields{
						"error": err.Error(),
						"url":   k.url,
					})
				}
				k.scheduleReconnect()
				return
			}

			if err := k.handleMessage(messageBytes); err != nil {
				logging.Warn(context.Background(), "Error handling WebSocket message", logging.Fields{
					"error": err.Error(),
					"url":   k.url,
				})
			}
		}
	}
}

// handleMessage procesa los mensajes recibidos del WebSocket
func (k *WebSocketClient) handleMessage(messageBytes []byte) error {
	// Intentar parsear como array (actualizaciones de ticker)
	var tickerArray []interface{}
	if err := json.Unmarshal(messageBytes, &tickerArray); err == nil && len(tickerArray) >= 4 {
		return k.handleTickerUpdate(tickerArray)
	}

	// Intentar parsear como mensaje de evento
	var msg WebSocketMessage
	if err := json.Unmarshal(messageBytes, &msg); err == nil {
		return k.handleEventMessage(msg)
	}

	return nil
}

// handleTickerUpdate procesa actualizaciones de ticker
func (k *WebSocketClient) handleTickerUpdate(data []interface{}) error {
	if len(data) < 4 {
		return fmt.Errorf("invalid ticker update format")
	}

	// Kraken ticker format: [channelID, tickerData, channelName, pair]
	tickerDataInterface, ok := data[1].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid ticker data format")
	}

	pairInterface, ok := data[3].(string)
	if !ok {
		return fmt.Errorf("invalid pair format")
	}

	// Extraer el precio de la última transacción
	lastTradeInterface, ok := tickerDataInterface["c"]
	if !ok {
		return fmt.Errorf("no last trade data found")
	}

	lastTradeArray, ok := lastTradeInterface.([]interface{})
	if !ok || len(lastTradeArray) == 0 {
		return fmt.Errorf("invalid last trade format")
	}

	priceStr, ok := lastTradeArray[0].(string)
	if !ok {
		return fmt.Errorf("invalid price format")
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse price: %w", err)
	}

	// Encontrar el par original
	// Intentar primero con formato WS (XBT/USD)
	originalPair, wsErr := fromWebSocketPair(pairInterface)
	if wsErr != nil {
		// Fallback a lógica previa basada en códigos internos
		originalPair = k.findOriginalPairFromKraken(pairInterface)
	}
	if originalPair == "" {
		return fmt.Errorf("unknown pair: %s", pairInterface)
	}

	// Crear entidad Price y enviar al canal
	priceEntity := entities.NewPrice(
		originalPair,
		price,
		time.Now(),
		0,
	)

	// Actualizar cache global
	if k.cache != nil {
		_ = k.cache.Set(context.Background(), priceEntity)
	}

	// Usar defer recover para manejar el caso de canal cerrado
	defer func() {
		if r := recover(); r != nil {
			// Canal cerrado, ignorar silenciosamente
			logging.Debug(context.Background(), "Channel closed during send", logging.Fields{
				"pair": originalPair,
			})
		}
	}()

	// Enviar de forma no bloqueante con acceso seguro
	k.mu.RLock()
	priceChan, exists := k.priceChannels[originalPair]
	k.mu.RUnlock()

	if !exists {
		// Par no suscrito, ignorar actualización
		return nil
	}

	select {
	case priceChan <- priceEntity:
		// Enviado exitosamente
	default:
		// Canal lleno, descartar precio más antiguo y registrar métrica
		metrics.RecordWebSocketChannelDrop(originalPair)
		select {
		case <-priceChan:
			// Descartado precio antiguo
		default:
			// Canal aún lleno, pero continuamos
		}
		// Intentar enviar el nuevo precio
		select {
		case priceChan <- priceEntity:
			// Enviado después de limpiar
		default:
			// Canal sigue lleno, descartamos la actualización
		}
	}

	return nil
}

// handleEventMessage procesa mensajes de eventos (suscripciones, errores, etc.)
func (k *WebSocketClient) handleEventMessage(msg WebSocketMessage) error {
	switch msg.Event {
	case "subscriptionStatus":
		switch msg.Status {
		case "subscribed":
			logging.Info(context.Background(), "Successfully subscribed to ticker for pairs", logging.Fields{
				"pairs": msg.Pair,
				"url":   k.url,
			})
		case "error":
			return fmt.Errorf("subscription error: %s", msg.ErrorMessage)
		}
	case "systemStatus":
		logging.Info(context.Background(), "Kraken WebSocket system status", logging.Fields{
			"status": msg.Status,
			"url":    k.url,
		})
	}
	return nil
}

// pingHandler envía pings periódicos para mantener la conexión activa
func (k *WebSocketClient) pingHandler() {
	defer k.wg.Done()
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.mu.RLock()
			conn := k.conn
			k.mu.RUnlock()
			if conn == nil {
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				k.scheduleReconnect()
				return
			}
		}
	}
}

// scheduleReconnect programa un intento de reconexión con gestión de estado mejorada
func (k *WebSocketClient) scheduleReconnect() {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Prevenir múltiples reconexiones concurrentes
	if !k.isConnected || k.isReconnecting {
		return
	}

	k.isConnected = false
	k.isReconnecting = true
	k.reconnectCount++

	// Implementar backoff exponencial con máximo de 60 segundos
	delay := time.Duration(k.reconnectCount) * time.Second
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}

	// Límite máximo de reintentos para evitar reconexión infinita
	if k.reconnectCount > 10 {
		logging.Error(context.Background(), "Maximum WebSocket reconnection attempts reached", logging.Fields{
			"max_attempts": k.reconnectCount,
			"url":          k.url,
		})
		k.isReconnecting = false
		return
	}

	logging.Info(context.Background(), "Scheduling WebSocket reconnection", logging.Fields{
		"delay_seconds": delay.Seconds(),
		"attempt":       k.reconnectCount,
		"url":           k.url,
	})

	k.reconnectTimer = time.AfterFunc(delay, func() {
		k.performReconnect()
	})
}

// performReconnect ejecuta el intento de reconexión de forma thread-safe
func (k *WebSocketClient) performReconnect() {
	// Verificar si debemos continuar (puede haberse cerrado mientras esperábamos)
	k.mu.RLock()
	shouldContinue := k.isReconnecting && k.ctx.Err() == nil
	k.mu.RUnlock()

	if !shouldContinue {
		return
	}

	logging.Info(context.Background(), "Attempting WebSocket reconnection", logging.Fields{
		"attempt": k.reconnectCount,
		"url":     k.url,
	})

	if err := k.Connect(); err != nil {
		logging.Warn(context.Background(), "WebSocket reconnection attempt failed", logging.Fields{
			"attempt": k.reconnectCount,
			"error":   err.Error(),
			"url":     k.url,
		})
		// Programar siguiente intento
		k.scheduleReconnect()
	} else {
		logging.Info(context.Background(), "WebSocket reconnected successfully", logging.Fields{
			"attempts_taken": k.reconnectCount,
			"url":            k.url,
		})

		// Reset del contador de reconexión
		k.mu.Lock()
		k.isReconnecting = false
		k.reconnectCount = 0
		k.mu.Unlock()

		// Re-subscribe to all pairs de forma segura
		k.mu.RLock()
		var pairs []string
		for pair := range k.subscriptions {
			pairs = append(pairs, pair)
		}
		k.mu.RUnlock()

		if len(pairs) > 0 {
			if err := k.SubscribeTicker(pairs); err != nil {
				// Intentar suscripción individual por par para aislar fallos
				var failed []string
				for _, p := range pairs {
					if subErr := k.SubscribeTicker([]string{p}); subErr != nil {
						failed = append(failed, p)
						logging.Warn(context.Background(), "Failed to re-subscribe individual pair after reconnect", logging.Fields{
							"pair":  p,
							"error": subErr.Error(),
							"url":   k.url,
						})
					}
				}

				if len(failed) > 0 {
					logging.Error(context.Background(), "Re-subscription completed with failures", logging.Fields{
						"failed_pairs": failed,
						"failed_count": len(failed),
						"url":          k.url,
					})
				} else {
					logging.Info(context.Background(), "Successfully re-subscribed all pairs after granular retry", logging.Fields{
						"pairs_count": len(pairs),
						"url":         k.url,
					})
				}
			} else {
				logging.Info(context.Background(), "Successfully re-subscribed to pairs after reconnect", logging.Fields{
					"pairs_count": len(pairs),
					"pairs":       pairs,
					"url":         k.url,
				})
			}
		}
	}
}

// findOriginalPairFromKraken encuentra el par original basado en el nombre de Kraken
func (k *WebSocketClient) findOriginalPairFromKraken(krakenPair string) string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	for originalPair := range k.subscriptions {
		if krakenConverted, err := toKrakenPair(originalPair); err == nil && krakenConverted == krakenPair {
			return originalPair
		}
	}

	// Intentar conversión inversa usando FromKrakenPair
	if friendlyPair, err := FromKrakenPair(krakenPair); err == nil {
		return friendlyPair
	}

	return ""
}

// IsConnected retorna el estado de conexión del WebSocket de forma thread-safe
func (k *WebSocketClient) IsConnected() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.isConnected
}

// GetReconnectionStatus retorna información sobre el estado de reconexión
func (k *WebSocketClient) GetReconnectionStatus() (isReconnecting bool, attemptCount int) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.isReconnecting, k.reconnectCount
}

// GetPriceCache expone el adaptador de cache para uso externo (por ejemplo, FallbackExchange)
func (k *WebSocketClient) GetPriceCache() *cachepkg.PriceCacheAdapter {
	return k.cache
}
