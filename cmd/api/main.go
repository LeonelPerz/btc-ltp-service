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
	"btc-ltp-service/internal/config"
	"btc-ltp-service/internal/handler"
	"btc-ltp-service/internal/logger"
	"btc-ltp-service/internal/metrics"
	"btc-ltp-service/internal/model"
	"btc-ltp-service/internal/service"
)

func main() {
	log.Println("Starting BTC Last Traded Price Service...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize supported pairs based on configuration
	if err := model.InitializeSupportedPairs(cfg.App.SupportedPairs); err != nil {
		log.Fatalf("Failed to initialize supported pairs: %v", err)
	}

	log.Printf("Initialized %d supported pairs: %v", len(cfg.App.SupportedPairs), cfg.App.SupportedPairs)

	// Configure structured logging
	logger.SetLogLevel(cfg.App.LogLevel)
	structuredLogger := logger.GetLogger()

	// Create root context for background operations
	ctx := context.Background()
	ctx = logger.WithRequestID(ctx)

	// Initialize components
	structuredLogger.Info("Initializing service components...")

	// Create Kraken hybrid client with WebSocket and REST fallback
	krakenClient := kraken.NewHybridClient(cfg.Kraken)

	// Import metrics to initialize them
	_ = metrics.HTTPRequestsTotal

	// Create cache based on configuration
	cacheConfig := cache.CacheConfig{
		TTL:           cfg.Cache.TTL,
		RedisAddr:     cfg.Redis.Addr,
		RedisPassword: cfg.Redis.Password,
		RedisDB:       cfg.Redis.DB,
	}

	priceCache, err := cache.NewCacheFromConfig(cfg.Cache.Backend, cacheConfig)
	if err != nil {
		structuredLogger.WithField("error", err.Error()).Fatal("Failed to create cache")
	}
	defer priceCache.Close()

	structuredLogger.WithField("backend", cfg.Cache.Backend).Info("Cache initialized successfully")

	// Set service info metrics
	metrics.SetServiceInfo("1.0.0", cfg.Cache.Backend)

	// Create LTP service
	ltpService := service.NewLTPService(krakenClient, priceCache)

	// Ensure proper cleanup of connections
	defer ltpService.Close()

	// Create HTTP handler
	ltpHandler := handler.NewLTPHandler(ltpService)

	// Create HTTP server
	server := handler.CreateServer(ltpHandler, cfg.Server.Port)

	structuredLogger.WithField("port", cfg.Server.Port).Info("Server starting")

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			structuredLogger.WithField("error", err.Error()).Fatal("Failed to start server")
		}
	}()

	// Start WebSocket connection for real-time updates
	structuredLogger.Info("Starting WebSocket connection for real-time price updates...")
	if err := ltpService.StartWebSocketConnection(); err != nil {
		structuredLogger.WithField("error", err.Error()).Warn("Failed to start WebSocket connection, will use REST fallback")
	} else {
		structuredLogger.Info("WebSocket connection initialized successfully")
	}

	// Pre-warm cache with initial data
	structuredLogger.Info("Pre-warming cache with initial price data...")
	if err := ltpService.RefreshAllPrices(); err != nil {
		structuredLogger.WithField("error", err.Error()).Warn("Failed to pre-warm cache")
	} else {
		structuredLogger.Info("Cache pre-warmed successfully")
	}

	// Start background price refresh routine
	go startPriceRefreshRoutine(ltpService, cfg.Cache.RefreshInterval, ctx)

	// Log connection status
	connectionStatus := ltpService.GetConnectionStatus()

	structuredLogger.WithFields(map[string]interface{}{
		"port": cfg.Server.Port,
		"endpoints": map[string]string{
			"health":  "/health",
			"ltp":     "/api/v1/ltp",
			"pairs":   "/api/v1/pairs",
			"status":  "/api/v1/status",
			"metrics": "/metrics",
		},
		"websocket_enabled":   connectionStatus["websocket_enabled"],
		"websocket_connected": connectionStatus["websocket_connected"],
		"fallback_mode":       connectionStatus["fallback_mode"],
	}).Info("BTC LTP Service is running")

	log.Printf("BTC LTP Service is running on http://localhost:%s", cfg.Server.Port)
	log.Printf("Health check available at: http://localhost:%s/health", cfg.Server.Port)
	log.Printf("LTP endpoint available at: http://localhost:%s/api/v1/ltp", cfg.Server.Port)
	log.Printf("Supported pairs endpoint available at: http://localhost:%s/api/v1/pairs", cfg.Server.Port)
	log.Printf("Connection status endpoint available at: http://localhost:%s/api/v1/status", cfg.Server.Port)
	log.Printf("Metrics available at: http://localhost:%s/metrics", cfg.Server.Port)

	// Log WebSocket status
	if connectionStatus["websocket_enabled"].(bool) {
		if connectionStatus["websocket_connected"].(bool) {
			log.Printf("✓ WebSocket connection: ACTIVE (real-time updates)")
		} else {
			log.Printf("✗ WebSocket connection: INACTIVE (using REST fallback)")
		}
	} else {
		log.Printf("- WebSocket: DISABLED (REST-only mode)")
	}

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	structuredLogger.Info("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(shutdownCtx); err != nil {
		structuredLogger.WithField("error", err.Error()).Fatal("Server forced to shutdown")
	}

	structuredLogger.Info("Server shutdown completed")
}

// startPriceRefreshRoutine starts a background routine to refresh prices periodically
func startPriceRefreshRoutine(ltpService *service.LTPService, refreshInterval time.Duration, ctx context.Context) {
	// Refresh prices at configured intervals to ensure cache stays fresh
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	structuredLogger := logger.GetLogger()
	structuredLogger.WithField("interval", refreshInterval.String()).Info("Starting background price refresh routine")

	for {
		select {
		case <-ticker.C:
			if err := ltpService.RefreshAllPrices(); err != nil {
				structuredLogger.WithField("error", err.Error()).Error("Background price refresh failed")
			} else {
				structuredLogger.Info("Background price refresh completed")
			}
		}
	}
}
