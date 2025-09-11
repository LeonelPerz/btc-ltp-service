package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"btc-ltp-service/internal/cache"
	"btc-ltp-service/internal/client/kraken"
	"btc-ltp-service/internal/handler"
	"btc-ltp-service/internal/service"
)

const (
	DefaultPort = "8080"
	ShutdownTimeout = 30 * time.Second
)

func main() {
	log.Println("Starting BTC Last Traded Price Service...")

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	// Initialize components
	log.Println("Initializing service components...")

	// Create Kraken API client
	krakenClient := kraken.NewClient()

	// Create price cache
	priceCache := cache.NewPriceCache()

	// Create LTP service
	ltpService := service.NewLTPService(krakenClient, priceCache)

	// Create HTTP handler
	ltpHandler := handler.NewLTPHandler(ltpService)

	// Create HTTP server
	server := handler.CreateServer(ltpHandler, port)

	log.Printf("Server starting on port %s", port)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Pre-warm cache with initial data
	log.Println("Pre-warming cache with initial price data...")
	if err := ltpService.RefreshAllPrices(); err != nil {
		log.Printf("Warning: Failed to pre-warm cache: %v", err)
	} else {
		log.Println("Cache pre-warmed successfully")
	}

	// Start background price refresh routine
	go startPriceRefreshRoutine(ltpService)

	log.Printf("BTC LTP Service is running on http://localhost:%s", port)
	log.Printf("Health check available at: http://localhost:%s/health", port)
	log.Printf("LTP endpoint available at: http://localhost:%s/api/v1/ltp", port)
	log.Printf("Supported pairs endpoint available at: http://localhost:%s/api/v1/pairs", port)

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server shutdown completed")
}

// startPriceRefreshRoutine starts a background routine to refresh prices periodically
func startPriceRefreshRoutine(ltpService *service.LTPService) {
	// Refresh prices every 30 seconds to ensure cache stays fresh
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("Starting background price refresh routine (every 30 seconds)")

	for {
		select {
		case <-ticker.C:
			if err := ltpService.RefreshAllPrices(); err != nil {
				log.Printf("Background refresh failed: %v", err)
			} else {
				log.Println("Background price refresh completed")
			}
		}
	}
}
