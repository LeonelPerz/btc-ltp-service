package interfaces

import "btc-ltp-service/internal/domain/entities"

type Exchange interface {
	GetTickers(pairs []string) ([]*entities.Price, error)
	GetTicker(pair string) (*entities.Price, error)
}
