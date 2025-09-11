package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"btc-ltp-service/internal/service"
)

// LTPHandler handles HTTP requests for Last Traded Price endpoints
type LTPHandler struct {
	ltpService *service.LTPService
}

// NewLTPHandler creates a new LTP handler instance
func NewLTPHandler(ltpService *service.LTPService) *LTPHandler {
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

	// Log the request
	if len(pairs) > 0 {
		log.Printf("LTP request for pairs: %v", pairs)
	} else {
		log.Printf("LTP request for all supported pairs")
	}

	// Get LTP data from service
	ltpResponse, err := h.ltpService.GetLTP(pairs)
	if err != nil {
		log.Printf("Error getting LTP data: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve price data")
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60") // Cache for 1 minute

	// Write JSON response
	if err := json.NewEncoder(w).Encode(ltpResponse); err != nil {
		log.Printf("Error encoding response: %v", err)
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	log.Printf("Successfully served LTP data for %d pairs", len(ltpResponse.LTP))
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
		"status": "healthy",
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

// parsePairsFromQuery extracts trading pairs from query parameters
// Supports both "pair" and "pairs" query parameters
// Examples: 
//   ?pair=BTC/USD
//   ?pairs=BTC/USD,BTC/EUR
//   ?pair=BTC/USD&pair=BTC/EUR
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
	mux.HandleFunc("/api/v1/ltp", h.HandleLTP)
	mux.HandleFunc("/health", h.HandleHealth)
	mux.HandleFunc("/api/v1/pairs", h.HandleSupportedPairs)
}

// loggingMiddleware logs HTTP requests
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
	h = loggingMiddleware(h)

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: h,
	}
}
