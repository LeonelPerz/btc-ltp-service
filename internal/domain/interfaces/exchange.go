package interfaces

import (
	"btc-ltp-service/internal/domain/entities"
	"context"
)

type Exchange interface {
	GetTickers(ctx context.Context, pairs []string) ([]*entities.Price, error)
	GetTicker(ctx context.Context, pair string) (*entities.Price, error)
}
