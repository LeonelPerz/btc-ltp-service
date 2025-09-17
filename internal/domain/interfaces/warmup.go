package interfaces

import (
	"btc-ltp-service/internal/domain/entities"
	"context"
)

// WarmupExchange define una operación rápida para precargar precios
// sin depender de WebSocket; típicamente implementada usando REST.
// Esto se usa en la fase de arranque para llenar la caché.
type WarmupExchange interface {
	WarmupTickers(ctx context.Context, pairs []string) ([]*entities.Price, error)
}
