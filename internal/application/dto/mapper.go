package dto

import (
	"btc-ltp-service/internal/domain/entities"
	"sort"
)

// PriceMapper maneja la conversión entre entidades del dominio y DTOs
type PriceMapper struct{}

// NewPriceMapper crea una nueva instancia del mapper
func NewPriceMapper() *PriceMapper {
	return &PriceMapper{}
}

// ToGetLTPResponse convierte una lista de precios del dominio a DTO de respuesta
func (m *PriceMapper) ToGetLTPResponse(prices []*entities.Price) *GetLTPResponse {
	if len(prices) == 0 {
		return &GetLTPResponse{
			LTP: []PriceData{},
		}
	}

	// Convertir cada precio
	priceData := make([]PriceData, len(prices))
	for i, price := range prices {
		priceData[i] = m.toPriceData(price)
	}

	// Ordenar por pair para respuesta consistente
	sort.Slice(priceData, func(i, j int) bool {
		return priceData[i].Pair < priceData[j].Pair
	})

	return &GetLTPResponse{
		LTP: priceData,
	}
}

// toPriceData convierte una entidad Price a PriceData DTO
func (m *PriceMapper) toPriceData(price *entities.Price) PriceData {
	return PriceData{
		Pair:   price.Pair,
		Amount: price.Amount,
	}
}

// FilterPricesByPairs filtra precios basado en los pares solicitados
func (m *PriceMapper) FilterPricesByPairs(prices []*entities.Price, requestedPairs []string) []*entities.Price {
	if len(requestedPairs) == 0 {
		return prices
	}

	// Crear mapa para lookup eficiente
	pairMap := make(map[string]bool)
	for _, pair := range requestedPairs {
		pairMap[pair] = true
	}

	// Filtrar precios
	var filtered []*entities.Price
	for _, price := range prices {
		if pairMap[price.Pair] {
			filtered = append(filtered, price)
		}
	}

	return filtered
}

// ValidateRequestedPairs valida que los pares solicitados sean válidos
func (m *PriceMapper) ValidateRequestedPairs(pairs []string) error {
	request := &GetLTPRequest{Pairs: pairs}
	return request.Validate()
}
