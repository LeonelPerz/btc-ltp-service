package exchange

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/domain/interfaces"
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/exchange/kraken"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/metrics"
	"context"
	"fmt"
	"strings"
	"time"
)

// FallbackExchange implementa la interfaz Exchange con estrategia de fallback
// WebSocket → REST para garantizar alta disponibilidad, usando configuración inyectada
type FallbackExchange struct {
	primary   *kraken.WebSocketClient // Cliente WebSocket (preferido)
	secondary interfaces.Exchange     // Cliente REST (fallback)
	config    config.KrakenConfig     // Configuración de Kraken
}

// NewFallbackExchange crea una nueva instancia del exchange con fallback usando configuración y lista de pares a suscribir al inicio
func NewFallbackExchange(krakenConfig config.KrakenConfig, supportedPairs []string) *FallbackExchange {
	// Crear clientes con configuración
	wsClient := kraken.NewWebSocketClientWithConfig(krakenConfig)
	restClient := kraken.NewRestClientWithConfig(krakenConfig)

	exchange := &FallbackExchange{
		primary:   wsClient,
		secondary: restClient,
		config:    krakenConfig,
	}

	// Intentar conectar WebSocket al inicio de forma asíncrona con contexto controlado
	go func() {
		// Crear contexto con timeout para evitar bloqueos indefinidos
		ctx, cancel := context.WithTimeout(context.Background(), krakenConfig.FallbackTimeout)
		defer cancel()

		// Usar canal para hacer la conexión cancelable
		done := make(chan error, 1)
		go func() {
			done <- wsClient.Connect()
		}()

		select {
		case err := <-done:
			if err != nil {
				metrics.UpdateWebSocketConnectionStatus(false)
				metrics.RecordWebSocketReconnectionAttempt("startup")
				logging.Warn(ctx, "Failed to initialize WebSocket connection at startup", logging.Fields{
					"error":            err.Error(),
					"websocket_url":    krakenConfig.WebSocketURL,
					"fallback_timeout": krakenConfig.FallbackTimeout,
				})
			} else {
				metrics.UpdateWebSocketConnectionStatus(true)
				logging.Info(ctx, "WebSocket connection established successfully", logging.Fields{
					"websocket_url": krakenConfig.WebSocketURL,
				})

				// Suscribir pares soportados inmediatamente
				if len(supportedPairs) > 0 {
					if subErr := wsClient.SubscribeTicker(supportedPairs); subErr != nil {
						logging.Warn(ctx, "Failed to subscribe supported pairs on startup", logging.Fields{
							"error": subErr.Error(),
							"pairs": supportedPairs,
						})
					}

					// Lanzar watchdog de frescura
					go exchange.startStalenessWatcher(supportedPairs, 60*time.Second)
				}
			}
		case <-ctx.Done():
			metrics.UpdateWebSocketConnectionStatus(false)
			metrics.RecordWebSocketReconnectionAttempt("startup")
			logging.Warn(ctx, "WebSocket connection startup timeout", logging.Fields{
				"timeout":       krakenConfig.FallbackTimeout,
				"websocket_url": krakenConfig.WebSocketURL,
			})
		}
	}()

	return exchange
}

// GetTicker obtiene el precio de un par usando WebSocket con fallback a REST
func (f *FallbackExchange) GetTicker(ctx context.Context, pair string) (*entities.Price, error) {
	logging.Debug(ctx, "Attempting to get ticker with fallback strategy", logging.Fields{
		"pair":             pair,
		"fallback_timeout": f.config.FallbackTimeout,
	})

	// 0. Probar cache global antes de WebSocket
	if cache := f.primary.GetPriceCache(); cache != nil {
		if cached, ok := cache.Get(ctx, pair); ok {
			return cached, nil
		}
	}

	// 1. Intentar con WebSocket primero
	price, err := f.tryWebSocketSingle(ctx, pair, func(ctx context.Context) (*entities.Price, error) {
		return f.primary.GetTicker(ctx, pair)
	})

	if err == nil {
		logging.Debug(ctx, "Successfully retrieved price via WebSocket", logging.Fields{
			"pair":   pair,
			"amount": price.Amount,
			"source": "websocket",
		})
		return price, nil
	}

	// 2. Fallback a REST
	fallbackReason := f.determineFallbackReason(err)
	metrics.RecordFallbackActivation(fallbackReason, pair)

	logging.Info(ctx, "WebSocket failed, falling back to REST API", logging.Fields{
		"pair":             pair,
		"websocket_error":  err.Error(),
		"fallback_reason":  fallbackReason,
		"fallback_timeout": f.config.FallbackTimeout,
	})

	fallbackStartTime := time.Now()
	restStartTime := time.Now()
	price, restErr := f.secondary.GetTicker(ctx, pair)
	restDuration := time.Since(restStartTime)

	if restErr != nil {
		logging.Error(ctx, "Both WebSocket and REST failed", logging.Fields{
			"pair":             pair,
			"websocket_error":  err.Error(),
			"rest_error":       restErr.Error(),
			"rest_duration_ms": restDuration.Milliseconds(),
		})
		return nil, fmt.Errorf("both WebSocket and REST failed - WebSocket: %v, REST: %v", err, restErr)
	}

	// Record successful fallback duration
	fallbackDuration := time.Since(fallbackStartTime)
	metrics.RecordFallbackDuration(pair, fallbackDuration.Seconds())

	logging.Info(ctx, "Successfully retrieved price via REST fallback", logging.Fields{
		"pair":                 pair,
		"amount":               price.Amount,
		"source":               "rest_fallback",
		"rest_duration_ms":     restDuration.Milliseconds(),
		"fallback_duration_ms": fallbackDuration.Milliseconds(),
	})

	return price, nil
}

// GetTickers obtiene precios múltiples usando WebSocket con fallback a REST
func (f *FallbackExchange) GetTickers(ctx context.Context, pairs []string) ([]*entities.Price, error) {
	if len(pairs) == 0 {
		return []*entities.Price{}, nil
	}

	logging.Debug(ctx, "Attempting to get multiple tickers with fallback strategy", logging.Fields{
		"pairs_count":      len(pairs),
		"pairs":            pairs,
		"fallback_timeout": f.config.FallbackTimeout,
	})

	// 0. Intentar cache global primero
	var cached []*entities.Price
	var missing []string
	if cache := f.primary.GetPriceCache(); cache != nil {
		cached, missing = cache.GetMany(ctx, pairs)
		if len(missing) == 0 {
			return cached, nil
		}
	} else {
		missing = pairs
	}

	// 1. Intentar con WebSocket para pares faltantes
	pricesMissing, err := f.tryWebSocketMultiple(ctx, "multiple_pairs", func(ctx context.Context) ([]*entities.Price, error) {
		return f.primary.GetTickers(ctx, missing)
	})

	prices := append(cached, pricesMissing...)

	if err == nil {
		logging.Debug(ctx, "Successfully retrieved prices via WebSocket", logging.Fields{
			"pairs_count":     len(pairs),
			"retrieved_count": len(prices),
			"source":          "websocket",
		})
		return prices, nil
	}

	// 2. Fallback a REST
	fallbackReason := f.determineFallbackReason(err)
	// Record fallback activation for each pair
	for _, pair := range pairs {
		metrics.RecordFallbackActivation(fallbackReason, pair)
	}

	logging.Info(ctx, "WebSocket failed, falling back to REST API for multiple pairs", logging.Fields{
		"pairs_count":      len(pairs),
		"websocket_error":  err.Error(),
		"fallback_reason":  fallbackReason,
		"fallback_timeout": f.config.FallbackTimeout,
	})

	fallbackStartTime := time.Now()
	restStartTime := time.Now()
	prices, restErr := f.secondary.GetTickers(ctx, pairs)
	restDuration := time.Since(restStartTime)

	if restErr != nil {
		logging.Error(ctx, "Both WebSocket and REST failed for multiple pairs", logging.Fields{
			"pairs_count":      len(pairs),
			"websocket_error":  err.Error(),
			"rest_error":       restErr.Error(),
			"rest_duration_ms": restDuration.Milliseconds(),
		})
		return nil, fmt.Errorf("both WebSocket and REST failed for multiple pairs - WebSocket: %v, REST: %v", err, restErr)
	}

	// Record successful fallback duration for each pair
	fallbackDuration := time.Since(fallbackStartTime)
	for _, price := range prices {
		if price != nil {
			metrics.RecordFallbackDuration(price.Pair, fallbackDuration.Seconds())
		}
	}

	logging.Info(ctx, "Successfully retrieved prices via REST fallback", logging.Fields{
		"pairs_count":          len(pairs),
		"retrieved_count":      len(prices),
		"source":               "rest_fallback",
		"rest_duration_ms":     restDuration.Milliseconds(),
		"fallback_duration_ms": fallbackDuration.Milliseconds(),
	})

	return prices, nil
}

// tryWebSocketSingle intenta ejecutar una operación WebSocket para un solo precio con timeout configurado
func (f *FallbackExchange) tryWebSocketSingle(ctx context.Context, operation string, wsFunc func(context.Context) (*entities.Price, error)) (*entities.Price, error) {
	var lastErr error
	for attempt := 1; attempt <= f.config.MaxRetries; attempt++ {
		wsCtx, cancel := context.WithTimeout(ctx, f.config.FallbackTimeout)
		resultChan := make(chan *entities.Price, 1)
		errorChan := make(chan error, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					errorChan <- fmt.Errorf("WebSocket panic recovered: %v", r)
				}
			}()
			res, err := wsFunc(wsCtx)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- res
		}()

		select {
		case res := <-resultChan:
			cancel()
			return res, nil
		case err := <-errorChan:
			lastErr = err
		case <-wsCtx.Done():
			lastErr = fmt.Errorf("WebSocket timeout after %v for operation: %s", f.config.FallbackTimeout, operation)
		}
		cancel()
		logging.Warn(ctx, "WebSocket attempt failed", logging.Fields{
			"attempt":      attempt,
			"max_attempts": f.config.MaxRetries,
			"error":        lastErr.Error(),
		})
	}
	return nil, lastErr
}

// tryWebSocketMultiple intenta ejecutar una operación WebSocket para múltiples precios con timeout configurado
func (f *FallbackExchange) tryWebSocketMultiple(ctx context.Context, operation string, wsFunc func(context.Context) ([]*entities.Price, error)) ([]*entities.Price, error) {
	var lastErr error
	for attempt := 1; attempt <= f.config.MaxRetries; attempt++ {
		wsCtx, cancel := context.WithTimeout(ctx, f.config.FallbackTimeout)
		resultChan := make(chan []*entities.Price, 1)
		errorChan := make(chan error, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					errorChan <- fmt.Errorf("WebSocket panic recovered: %v", r)
				}
			}()
			res, err := wsFunc(wsCtx)
			if err != nil {
				errorChan <- err
				return
			}
			resultChan <- res
		}()

		select {
		case res := <-resultChan:
			cancel()
			return res, nil
		case err := <-errorChan:
			lastErr = err
		case <-wsCtx.Done():
			lastErr = fmt.Errorf("WebSocket timeout after %v for operation: %s", f.config.FallbackTimeout, operation)
		}
		cancel()
		logging.Warn(ctx, "WebSocket attempt failed", logging.Fields{
			"attempt":      attempt,
			"max_attempts": f.config.MaxRetries,
			"error":        lastErr.Error(),
		})
	}
	return nil, lastErr
}

// Close cierra las conexiones de ambos clientes
func (f *FallbackExchange) Close() error {
	var wsErr error

	if f.primary != nil {
		wsErr = f.primary.Close()
	}

	// El cliente REST no necesita cierre explícito

	if wsErr != nil {
		return fmt.Errorf("error closing WebSocket client: %w", wsErr)
	}

	logging.Info(context.Background(), "FallbackExchange closed successfully", nil)
	return nil
}

// GetPrimaryStatus retorna el estado real de la conexión WebSocket primaria
func (f *FallbackExchange) GetPrimaryStatus() bool {
	if f.primary == nil {
		return false
	}
	return f.primary.IsConnected()
}

// ForceWebSocketReconnect fuerza una reconexión del WebSocket (útil para testing/debugging)
func (f *FallbackExchange) ForceWebSocketReconnect() error {
	metrics.RecordWebSocketReconnectionAttempt("manual")
	metrics.UpdateWebSocketConnectionStatus(false)

	logging.Info(context.Background(), "Forcing WebSocket reconnection", logging.Fields{
		"websocket_url": f.config.WebSocketURL,
	})

	if err := f.primary.Close(); err != nil {
		logging.Warn(context.Background(), "Error closing WebSocket during forced reconnect", logging.Fields{
			"error": err.Error(),
		})
	}

	err := f.primary.Connect()
	if err == nil {
		metrics.UpdateWebSocketConnectionStatus(true)
	}

	return err
}

// GetConfig retorna la configuración actual (útil para debugging/monitoring)
func (f *FallbackExchange) GetConfig() config.KrakenConfig {
	return f.config
}

// WarmupTickers implementa interfaces.WarmupExchange devolviendo precios sólo vía REST
func (f *FallbackExchange) WarmupTickers(ctx context.Context, pairs []string) ([]*entities.Price, error) {
	return f.secondary.GetTickers(ctx, pairs)
}

// Secondary expone el cliente REST secundario (solo lectura)
func (f *FallbackExchange) Secondary() interfaces.Exchange {
	return f.secondary
}

// startStalenessWatcher verifica cada 20s que la edad del precio no supere maxAge;
// si sucede, actualiza vía REST y escribe en caché.
func (f *FallbackExchange) startStalenessWatcher(pairs []string, maxAge time.Duration) {
	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			for _, pair := range pairs {
				price, ok := f.primary.GetPriceCache().Get(ctx, pair)
				if ok && time.Since(price.Timestamp) <= maxAge {
					continue // todavía fresco
				}
				// fetch via REST
				p, err := f.secondary.GetTicker(ctx, pair)
				if err != nil {
					logging.Warn(ctx, "Staleness watcher REST fetch failed", logging.Fields{"pair": pair, "error": err.Error()})
					continue
				}
				_ = f.primary.GetPriceCache().Set(ctx, p)
				logging.Debug(ctx, "Staleness watcher refreshed price", logging.Fields{"pair": pair})
			}
			cancel()
		default:
			// Add small delay to prevent busy waiting
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// determineFallbackReason determines the reason for fallback based on error analysis
func (f *FallbackExchange) determineFallbackReason(err error) string {
	if err == nil {
		return "unknown"
	}

	errStr := strings.ToLower(err.Error())

	// Analyze error message to determine reason
	if strings.Contains(errStr, "timeout") {
		return "timeout"
	}
	if strings.Contains(errStr, "connection") {
		return "connection_error"
	}
	if strings.Contains(errStr, "max retries") || strings.Contains(errStr, "retries") {
		return "max_retries"
	}
	if strings.Contains(errStr, "panic") {
		return "panic"
	}
	if strings.Contains(errStr, "closed") || strings.Contains(errStr, "disconnected") {
		return "connection_closed"
	}

	return "unknown_error"
}
