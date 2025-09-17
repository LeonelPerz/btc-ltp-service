package dto

import (
	"errors"
	"strings"
)

// GetLTPRequest representa la request para obtener Last Traded Prices
type GetLTPRequest struct {
	// Pairs es la lista de pares solicitados (ej: "BTC/USD,ETH/USD" o "BTC/USD")
	Pairs []string `json:"pairs"`
}

// NewGetLTPRequest crea una nueva request desde query parameters
// Si pairsParam está vacío, usa defaultPairs como fallback
// VALIDA que todos los pares solicitados estén en la lista de pares soportados
func NewGetLTPRequest(pairsParam string, supportedPairs []string) (*GetLTPRequest, error) {
	// Si no se proporciona el parámetro, usar pares soportados como fallback
	if pairsParam == "" {
		if len(supportedPairs) == 0 {
			return nil, errors.New("no supported pairs configured")
		}
		return &GetLTPRequest{
			Pairs: supportedPairs,
		}, nil
	}

	// Crear mapa de pares soportados para búsqueda eficiente
	supportedMap := make(map[string]bool)
	for _, supportedPair := range supportedPairs {
		supportedMap[strings.ToUpper(supportedPair)] = true
	}

	// Split por comas y limpiar espacios
	pairsList := strings.Split(pairsParam, ",")
	var cleanPairs []string

	for _, pair := range pairsList {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Validación básica del formato (debe tener /)
		if !strings.Contains(pair, "/") {
			return nil, errors.New("invalid pair format: " + pair + " (expected BASE/QUOTE)")
		}

		// Normalizar a mayúsculas
		pair = strings.ToUpper(pair)

		// VALIDACIÓN CRÍTICA: Verificar que el par esté soportado
		if !supportedMap[pair] {
			return nil, errors.New("unsupported pair: " + pair + " (supported pairs: " + strings.Join(supportedPairs, ",") + ")")
		}

		cleanPairs = append(cleanPairs, pair)
	}

	if len(cleanPairs) == 0 {
		return nil, errors.New("no valid pairs provided")
	}

	return &GetLTPRequest{
		Pairs: cleanPairs,
	}, nil
}

// Validate valida la request
func (r *GetLTPRequest) Validate() error {
	if len(r.Pairs) == 0 {
		return errors.New("at least one pair is required")
	}

	for _, pair := range r.Pairs {
		if !strings.Contains(pair, "/") {
			return errors.New("invalid pair format: " + pair)
		}

		parts := strings.Split(pair, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return errors.New("invalid pair format: " + pair)
		}
	}

	return nil
}
