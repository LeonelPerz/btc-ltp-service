package interfaces

import (
	"btc-ltp-service/internal/domain/entities"
	"context"
)

// PriceService define los casos de uso relacionados con precios de criptomonedas
type PriceService interface {
	// GetLastPrice obtiene el último precio de un par (CACHE-ONLY, sin fallback)
	// Para consultas HTTP - siempre retorna desde caché o error si no está disponible
	GetLastPrice(ctx context.Context, pair string) (*entities.Price, error)

	// RefreshPrices actualiza el cache con precios frescos de múltiples pares
	// Usado SOLO por: 1) inicialización, 2) proceso automático cada 30s
	RefreshPrices(ctx context.Context, pairs []string) error

	// GetCachedPrices retorna todos los precios que están actualmente en cache
	GetCachedPrices(ctx context.Context) ([]*entities.Price, error)
}
