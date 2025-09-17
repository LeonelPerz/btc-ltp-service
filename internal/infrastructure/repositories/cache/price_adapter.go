package cache

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/domain/interfaces"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PriceCacheAdapter permite almacenar entidades Price en cualquier interfaces.Cache
// utilizando la clave price:<pair> y TTL configurable.
type PriceCacheAdapter struct {
	backend interfaces.Cache
	ttl     time.Duration
}

// NewPriceCache crea un nuevo adaptador.
func NewPriceCache(backend interfaces.Cache, ttl time.Duration) *PriceCacheAdapter {
	return &PriceCacheAdapter{
		backend: backend,
		ttl:     ttl,
	}
}

func (p *PriceCacheAdapter) key(pair string) string {
	return fmt.Sprintf("price:%s", pair)
}

// Set guarda el precio para un par.
func (p *PriceCacheAdapter) Set(ctx context.Context, price *entities.Price) error {
	bytes, err := json.Marshal(price)
	if err != nil {
		return err
	}
	return p.backend.Set(ctx, p.key(price.Pair), string(bytes), p.ttl)
}

// Get obtiene el precio si existe y no expiró.
func (p *PriceCacheAdapter) Get(ctx context.Context, pair string) (*entities.Price, bool) {
	str, err := p.backend.Get(ctx, p.key(pair))
	if err != nil {
		return nil, false
	}
	var price entities.Price
	if err := json.Unmarshal([]byte(str), &price); err != nil {
		return nil, false
	}
	return &price, true
}

// GetMany devuelve los precios existentes y la lista de pares faltantes.
func (p *PriceCacheAdapter) GetMany(ctx context.Context, pairs []string) ([]*entities.Price, []string) {
	prices := make([]*entities.Price, 0, len(pairs))
	missing := make([]string, 0)
	for _, pr := range pairs {
		if price, ok := p.Get(ctx, pr); ok {
			prices = append(prices, price)
		} else {
			missing = append(missing, pr)
		}
	}
	return prices, missing
}
