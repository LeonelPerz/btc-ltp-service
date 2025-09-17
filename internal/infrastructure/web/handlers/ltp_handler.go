package handlers

import (
	"btc-ltp-service/internal/application/dto"
	"btc-ltp-service/internal/domain/entities"
	"btc-ltp-service/internal/domain/interfaces"
	"btc-ltp-service/internal/infrastructure/logging"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// LTPHandler handles requests related to Last Traded Prices
type LTPHandler struct {
	priceService   interfaces.PriceService
	mapper         *dto.PriceMapper
	supportedPairs []string
}

// NewLTPHandler creates a new instance of the LTP handler
func NewLTPHandler(priceService interfaces.PriceService, supportedPairs []string) *LTPHandler {
	return &LTPHandler{
		priceService:   priceService,
		mapper:         dto.NewPriceMapper(),
		supportedPairs: supportedPairs,
	}
}

// GetLTP maneja GET /api/v1/ltp?pair=BTC/USD,ETH/USD
// Si no se proporciona el parámetro 'pair', devuelve todos los pares soportados
func (h *LTPHandler) GetLTP(w http.ResponseWriter, r *http.Request) {
	// 1. Parse query parameters (optional - if empty, use default pairs)
	pairsParam := r.URL.Query().Get("pair")

	// 2. Crear y validar request DTO con pares soportados como fallback
	request, err := dto.NewGetLTPRequest(pairsParam, h.supportedPairs)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	// 3. Get prices from service
	ctx := r.Context()

	logging.Info(ctx, "Fetching prices for pairs", logging.Fields{
		"pairs_count": len(request.Pairs),
		"pairs":       request.Pairs,
	})

	logging.Debug(ctx, "Handler GetLTP: About to call PriceService.GetLastPrice", logging.Fields{
		"method": "GetLastPrice",
		"pairs":  request.Pairs,
	})

	// Collect prices and errors separately for partial handling
	var allPrices []*entities.Price
	var priceErrors []dto.PriceError

	for _, pair := range request.Pairs {
		price, err := h.priceService.GetLastPrice(ctx, pair)
		if err != nil {
			logging.ErrorWithError(ctx, "Failed to get price for pair", err, logging.Fields{
				"pair": pair,
			})

			// Add specific error for this pair instead of failing the entire request
			priceErrors = append(priceErrors, dto.NewPriceError(
				pair,
				"Failed to fetch price",
				"PRICE_FETCH_ERROR",
				err.Error(),
			))
			continue // Continuar con los otros pares
		}

		allPrices = append(allPrices, price)
		logging.Debug(ctx, "Successfully retrieved price", logging.Fields{
			"pair":   pair,
			"amount": price.Amount,
			"age_ms": price.Age.Milliseconds(),
		})
	}

	// 4. Determine appropriate response based on successes and errors
	if len(priceErrors) == 0 {
		// All successful - clean response
		response := h.mapper.ToGetLTPResponse(allPrices)
		h.writeJSONResponseWithContext(w, r.Context(), http.StatusOK, response)
	} else if len(allPrices) == 0 {
		// All failed – indicar indisponibilidad del servicio backend
		logging.Error(ctx, "All price fetches failed", logging.Fields{
			"pairs_count":  len(request.Pairs),
			"errors_count": len(priceErrors),
		})

		response := dto.NewGetLTPResponseWithErrors(allPrices, priceErrors)
		h.writeJSONResponseWithContext(w, r.Context(), http.StatusServiceUnavailable, response)
	} else {
		// Partial success - response with included errors
		logging.Warn(ctx, "Partial success in price fetching", logging.Fields{
			"successful_count": len(allPrices),
			"failed_count":     len(priceErrors),
			"total_requested":  len(request.Pairs),
		})
		response := dto.NewGetLTPResponseWithErrors(allPrices, priceErrors)
		h.writeJSONResponseWithContext(w, r.Context(), http.StatusPartialContent, response)
	}
}

// RefreshPrices maneja POST /api/v1/ltp/refresh (para casos de administración)
func (h *LTPHandler) RefreshPrices(w http.ResponseWriter, r *http.Request) {

	pairsParam := r.URL.Query().Get("pairs")
	if pairsParam == "" {
		// Default to all supported pairs when not specified
		pairsParam = strings.Join(h.supportedPairs, ",")
	}

	request, err := dto.NewGetLTPRequest(pairsParam, h.supportedPairs)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	// Refresh prices
	ctx := r.Context()
	logging.Info(ctx, "Refreshing prices for pairs", logging.Fields{
		"pairs_count": len(request.Pairs),
		"pairs":       request.Pairs,
	})

	err = h.priceService.RefreshPrices(ctx, request.Pairs)

	response := map[string]interface{}{
		"pairs": request.Pairs,
	}

	if err != nil {
		logging.Warn(ctx, "Partial or failed refresh", logging.Fields{
			"error":       err.Error(),
			"pairs_count": len(request.Pairs),
		})
		response["message"] = "Prices refreshed with errors"
		response["error"] = err.Error()
	} else {
		logging.Info(ctx, "Successfully refreshed prices", logging.Fields{
			"pairs_count": len(request.Pairs),
		})
		response["message"] = "Prices refreshed successfully"
	}

	h.writeJSONResponseWithContext(w, r.Context(), http.StatusOK, response)
}

// GetCachedPrices maneja GET /api/v1/ltp/cached (para debugging/monitoring)
func (h *LTPHandler) GetCachedPrices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logging.Info(ctx, "Fetching cached prices", nil)

	cachedPrices, err := h.priceService.GetCachedPrices(ctx)
	if err != nil {
		logging.ErrorWithError(ctx, "Failed to get cached prices", err, nil)
		h.writeErrorResponse(w, http.StatusInternalServerError, "CACHE_ERROR", "Failed to get cached prices")
		return
	}

	logging.Info(ctx, "Successfully retrieved cached prices", logging.Fields{
		"cached_prices_count": len(cachedPrices),
	})

	// Convert to response DTO
	response := h.mapper.ToGetLTPResponse(cachedPrices)
	h.writeJSONResponseWithContext(w, ctx, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response (maintain backward compatibility)
func (h *LTPHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	ctx := w.Header().Get("X-Request-ID") // We can't access r.Context() here, so use request ID from header
	reqCtx := context.Background()
	if ctx != "" {
		reqCtx = logging.WithRequestID(reqCtx, ctx)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.ErrorWithError(reqCtx, "Failed to encode JSON response", err, logging.Fields{
			"status_code": statusCode,
		})
	}
}

// writeJSONResponseWithContext writes a JSON response preserving the original context
func (h *LTPHandler) writeJSONResponseWithContext(w http.ResponseWriter, ctx context.Context, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.ErrorWithError(ctx, "Failed to encode JSON response", err, logging.Fields{
			"status_code": statusCode,
		})
	}
}

// writeErrorResponse writes an error response
func (h *LTPHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	errorResp := dto.NewErrorResponseWithCode(errorCode, message, "")
	h.writeJSONResponse(w, statusCode, errorResp)
}
