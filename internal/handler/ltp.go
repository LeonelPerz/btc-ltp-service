package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"btc-ltp-service/internal/docs"
	"btc-ltp-service/internal/logger"
	"btc-ltp-service/internal/middleware"
	"btc-ltp-service/internal/model"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
)

// LTPServiceInterface defines the interface for LTP service operations
type LTPServiceInterface interface {
	GetLTP(pairs []string) (*model.LTPResponse, error)
	GetSupportedPairs() []string
	RefreshAllPrices() error
	GetConnectionStatus() map[string]interface{}
}

// LTPHandler handles HTTP requests for Last Traded Price endpoints
type LTPHandler struct {
	ltpService LTPServiceInterface
}

// NewLTPHandler creates a new LTP handler instance
func NewLTPHandler(ltpService LTPServiceInterface) *LTPHandler {
	return &LTPHandler{
		ltpService: ltpService,
	}
}

// HandleLTP handles the /api/v1/ltp endpoint
func (h *LTPHandler) HandleLTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters for pairs
	pairs := h.parsePairsFromQuery(r)

	// Get request context for structured logging
	ctx := r.Context()

	// Log the request details
	if len(pairs) > 0 {
		logger.LogServiceEvent(ctx, "ltp_request", "LTP request for specific pairs", map[string]interface{}{
			"pairs":      pairs,
			"pair_count": len(pairs),
		})
	} else {
		logger.LogServiceEvent(ctx, "ltp_request", "LTP request for all supported pairs", nil)
	}

	// Get LTP data from service
	ltpResponse, err := h.ltpService.GetLTP(pairs)
	if err != nil {
		logger.GetLogger().WithFields(map[string]interface{}{
			"request_id": logger.GetRequestID(ctx),
			"error":      err.Error(),
			"pairs":      pairs,
			"event":      "ltp_error",
		}).Error("Failed to get LTP data")
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve price data")
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60") // Cache for 1 minute

	// Write JSON response
	if err := json.NewEncoder(w).Encode(ltpResponse); err != nil {
		logger.GetLogger().WithFields(map[string]interface{}{
			"request_id": logger.GetRequestID(ctx),
			"error":      err.Error(),
			"event":      "encode_error",
		}).Error("Failed to encode response")
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	logger.LogServiceEvent(ctx, "ltp_success", "Successfully served LTP data", map[string]interface{}{
		"pairs_served": len(ltpResponse.LTP),
	})
}

// HandleHealth handles the health check endpoint
func (h *LTPHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status":  "healthy",
		"service": "btc-ltp-service",
	}

	json.NewEncoder(w).Encode(response)
}

// HandleSupportedPairs handles requests for supported trading pairs
func (h *LTPHandler) HandleSupportedPairs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	supportedPairs := h.ltpService.GetSupportedPairs()

	w.Header().Set("Content-Type", "application/json")

	response := map[string][]string{
		"supported_pairs": supportedPairs,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding supported pairs response: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}
}

// HandleConnectionStatus handles requests for connection status information
func (h *LTPHandler) HandleConnectionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := h.ltpService.GetConnectionStatus()

	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":     "ok",
		"connection": status,
		"timestamp":  time.Now().Unix(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding connection status response: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}
}

// parsePairsFromQuery extracts trading pairs from query parameters
// Supports both "pair" and "pairs" query parameters
// Examples:
//
//	?pair=BTC/USD
//	?pairs=BTC/USD,BTC/EUR
//	?pair=BTC/USD&pair=BTC/EUR
func (h *LTPHandler) parsePairsFromQuery(r *http.Request) []string {
	var pairs []string

	// Get "pair" parameters (can be multiple)
	pairParams := r.URL.Query()["pair"]
	pairs = append(pairs, pairParams...)

	// Get "pairs" parameter and split by comma
	pairsParam := r.URL.Query().Get("pairs")
	if pairsParam != "" {
		splitPairs := strings.Split(pairsParam, ",")
		for _, pair := range splitPairs {
			trimmedPair := strings.TrimSpace(pair)
			if trimmedPair != "" {
				pairs = append(pairs, trimmedPair)
			}
		}
	}

	// Remove duplicates and normalize
	uniquePairs := make([]string, 0)
	seen := make(map[string]bool)

	for _, pair := range pairs {
		normalizedPair := strings.ToUpper(strings.TrimSpace(pair))
		if normalizedPair != "" && !seen[normalizedPair] {
			uniquePairs = append(uniquePairs, normalizedPair)
			seen[normalizedPair] = true
		}
	}

	return uniquePairs
}

// writeErrorResponse writes an error response in JSON format
func (h *LTPHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
		},
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Failed to write error response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SetupRoutes sets up HTTP routes for the LTP service
func (h *LTPHandler) SetupRoutes(mux *http.ServeMux) {
	// API endpoints
	mux.HandleFunc("/api/v1/ltp", h.HandleLTP)
	mux.HandleFunc("/health", h.HandleHealth)
	mux.HandleFunc("/api/v1/pairs", h.HandleSupportedPairs)
	mux.HandleFunc("/api/v1/status", h.HandleConnectionStatus)

	// Monitoring endpoints
	mux.Handle("/metrics", promhttp.Handler())

	// Documentation endpoints
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	mux.HandleFunc("/swagger/doc.json", h.ServeSwaggerSpec)

	// Redirect /docs to /swagger/
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
}

// ServeSwaggerSpec serves the OpenAPI/Swagger specification
func (h *LTPHandler) ServeSwaggerSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Generate spec with current host
	spec := docs.SwaggerInfo_swagger

	// Replace template variables
	if r.Header.Get("X-Forwarded-Host") != "" {
		spec = strings.ReplaceAll(spec, "localhost:8080", r.Header.Get("X-Forwarded-Host"))
	} else if r.Host != "" {
		spec = strings.ReplaceAll(spec, "localhost:8080", r.Host)
	}

	// Replace other template variables
	spec = strings.ReplaceAll(spec, "{{.Title}}", docs.SwaggerInfo.Title)
	spec = strings.ReplaceAll(spec, "{{.Description}}", docs.SwaggerInfo.Description)
	spec = strings.ReplaceAll(spec, "{{.Version}}", docs.SwaggerInfo.Version)
	spec = strings.ReplaceAll(spec, "{{.Host}}", docs.SwaggerInfo.Host)
	spec = strings.ReplaceAll(spec, "{{.BasePath}}", docs.SwaggerInfo.BasePath)
	spec = strings.ReplaceAll(spec, "{{ marshal .Schemes }}", "[\"http\"]")
	spec = strings.ReplaceAll(spec, "{{escape .Description}}", docs.SwaggerInfo.Description)

	w.Write([]byte(spec))
}

// Deprecated: Use middleware.LoggingMiddleware instead
// loggingMiddleware logs HTTP requests (keeping for compatibility)
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CreateServer creates an HTTP server with middleware
func CreateServer(handler *LTPHandler, port string) *http.Server {
	mux := http.NewServeMux()
	handler.SetupRoutes(mux)

	// Apply middleware
	var h http.Handler = mux
	h = corsMiddleware(h)
	h = middleware.LoggingMiddleware(h) // Use new structured logging middleware
	h = middleware.MetricsMiddleware(h)

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: h,
	}
}
