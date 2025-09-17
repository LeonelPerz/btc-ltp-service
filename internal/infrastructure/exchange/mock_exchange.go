package exchange

import (
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/infrastructure/logging"
	"context"
	"fmt"
	"math/rand"
	"time"
)

// MockExchange implementa la interfaz Exchange para testing y development
// Retorna precios falsos pero realistas para facilitar el desarrollo
type MockExchange struct {
	basePrices map[string]float64 // Precios base para cada par
	variance   float64            // Variación porcentual para simular volatilidad
}

// NewMockExchange crea una nueva instancia del mock exchange
func NewMockExchange() *MockExchange {
	return &MockExchange{
		basePrices: map[string]float64{
			"BTC/USD": 65000.0, // Bitcoin ~ $65k
			"ETH/USD": 3200.0,  // Ethereum ~ $3.2k
			"LTC/USD": 95.0,    // Litecoin ~ $95
			"XRP/USD": 0.52,    // Ripple ~ $0.52
			"BTC/EUR": 59500.0, // Bitcoin in EUR
			"BTC/CHF": 58200.0, // Bitcoin in CHF
			"ETH/EUR": 2900.0,  // Ethereum in EUR
			"ETH/CHF": 2850.0,  // Ethereum in CHF
		},
		variance: 0.02, // ±2% variation
	}
}

// GetTicker retorna un precio falso para el par solicitado
func (m *MockExchange) GetTicker(ctx context.Context, pair string) (*entities.Price, error) {
	logging.Debug(ctx, "MockExchange: Getting ticker for pair", logging.Fields{
		"pair": pair,
	})

	basePrice, exists := m.basePrices[pair]
	if !exists {
		return nil, fmt.Errorf("unsupported trading pair: %s", pair)
	}

	// Simular volatilidad con variación aleatoria
	variation := (rand.Float64()*2 - 1) * m.variance // -variance% to +variance%
	currentPrice := basePrice * (1 + variation)

	// Simular "edad" del precio (entre 0-30 segundos)
	age := time.Duration(rand.Intn(30)) * time.Second

	price := entities.NewPrice(
		pair,
		currentPrice,
		time.Now().Add(-age), // Timestamp en el pasado para simular age
		age,
	)

	logging.Debug(ctx, "MockExchange: Generated mock price", logging.Fields{
		"pair":          pair,
		"base_price":    basePrice,
		"current_price": currentPrice,
		"variation":     fmt.Sprintf("%.2f%%", variation*100),
		"age_ms":        age.Milliseconds(),
	})

	return price, nil
}

// GetTickers retorna precios falsos para múltiples pares
func (m *MockExchange) GetTickers(ctx context.Context, pairs []string) ([]*entities.Price, error) {
	logging.Info(ctx, "MockExchange: Getting tickers for multiple pairs", logging.Fields{
		"pairs_count": len(pairs),
		"pairs":       pairs,
	})

	var prices []*entities.Price
	var errors []string

	for _, pair := range pairs {
		price, err := m.GetTicker(ctx, pair)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", pair, err))
			continue
		}
		prices = append(prices, price)
	}

	if len(errors) > 0 {
		logging.Warn(ctx, "MockExchange: Some pairs failed", logging.Fields{
			"successful_count": len(prices),
			"failed_count":     len(errors),
			"errors":           errors,
		})
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("failed to get prices for all pairs: %v", errors)
	}

	logging.Info(ctx, "MockExchange: Successfully generated mock prices", logging.Fields{
		"successful_count": len(prices),
		"failed_count":     len(errors),
	})

	return prices, nil
}

// AddPair agrega un nuevo par con precio base (útil para testing)
func (m *MockExchange) AddPair(pair string, basePrice float64) {
	m.basePrices[pair] = basePrice
}

// SetVariance configura la variación porcentual para volatilidad
func (m *MockExchange) SetVariance(variance float64) {
	m.variance = variance
}

// GetSupportedPairs retorna todos los pares soportados por el mock
func (m *MockExchange) GetSupportedPairs() []string {
	var pairs []string
	for pair := range m.basePrices {
		pairs = append(pairs, pair)
	}
	return pairs
}
