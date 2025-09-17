package handlers

import (
	"btc-ltp-service/internal/application/dto"
	"btc-ltp-service/internal/domain/interfaces"
	"encoding/json"
	"net/http"
)

// HealthHandler maneja los endpoints de health check
type HealthHandler struct {
	priceService interfaces.PriceService
}

// NewHealthHandler crea una nueva instancia del health handler
func NewHealthHandler(priceService interfaces.PriceService) *HealthHandler {
	return &HealthHandler{
		priceService: priceService,
	}
}

// Health godoc
// @Summary Basic health check
// @Description Verifies that the service is running correctly. Responds quickly without checking external dependencies.
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Service is running correctly"
// @Router /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{
		"service": "running",
	}

	response := dto.NewHealthResponse("healthy", services)
	h.writeJSONResponse(w, http.StatusOK, response)
}

// Ready godoc
// @Summary Complete readiness check
// @Description Verifies that the service is ready to receive traffic, including validation of dependencies like cache and external services.
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} dto.HealthResponse "Service is ready to receive traffic"
// @Failure 503 {object} dto.HealthResponse "Service is not ready - dependencies are failing"
// @Router /ready [get]
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services := make(map[string]string)

	// Test cache by getting cached prices
	_, err := h.priceService.GetCachedPrices(ctx)
	if err != nil {
		services["cache"] = "error: " + err.Error()
		response := dto.NewHealthResponse("unhealthy", services)
		h.writeJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	services["cache"] = "ready"
	services["service"] = "ready"

	response := dto.NewHealthResponse("ready", services)
	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse escribe una respuesta JSON
func (h *HealthHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, write basic error response
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"ENCODING_ERROR","message":"Failed to encode response"}`))
	}
}
