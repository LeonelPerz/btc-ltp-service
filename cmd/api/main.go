// BTC LTP Service API
//
// This is a comprehensive cryptocurrency Last Traded Price (LTP) service API
// that provides real-time price data with WebSocket fallback to REST API,
// intelligent caching, and enterprise-grade observability features.
//
// Features:
// - Real-time WebSocket price feeds with REST API fallback
// - Configurable caching with Memory or Redis backends
// - Comprehensive Prometheus metrics (50+ metrics)
// - Rate limiting with token bucket algorithm
// - Structured logging with request tracing
// - Health checks and readiness probes
// - Support for multiple cryptocurrency trading pairs
//
//	Schemes: http, https
//	Host: localhost:8080
//	BasePath: /api/v1
//	Version: 1.0.0
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
// swagger:meta
package main

import (
	"btc-ltp-service/internal/application/services"
	"btc-ltp-service/internal/domain/interfaces"
	"btc-ltp-service/internal/infrastructure/config"
	"btc-ltp-service/internal/infrastructure/exchange"
	"btc-ltp-service/internal/infrastructure/logging"
	"btc-ltp-service/internal/infrastructure/repositories/cache"
	"btc-ltp-service/internal/infrastructure/web/router"
	"btc-ltp-service/internal/infrastructure/web/server"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Application version
const AppVersion = "1.0.0"

func main() {
	ctx := context.Background()

	// 1. Load and validate configuration
	cfg, err := loadConfiguration(ctx)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize logging with configuration
	initializeLogging(ctx, cfg.Logging)

	logging.Info(ctx, "Starting BTC LTP Service", logging.Fields{
		"version":     AppVersion,
		"environment": config.GetEnvironment(),
		"log_level":   cfg.Logging.Level,
		"port":        cfg.Server.Port,
	})

	// 3. Initialize dependencies with configuration
	dependencies, err := initializeDependencies(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependencies: %v", err)
	}

	// 4. Pre-load cache with supported pairs
	err = initializeCacheWithSupportedPairs(ctx, dependencies.PriceService, dependencies.Exchange, cfg.Business.SupportedPairs)
	if err != nil {
		// Log warning but don't fail - service can work without initial cache
		logging.Warn(ctx, "Failed to initialize cache with supported pairs", logging.Fields{
			"error":                 err.Error(),
			"supported_pairs_count": len(cfg.Business.SupportedPairs),
			"supported_pairs":       cfg.Business.SupportedPairs,
		})
	}

	// 5. Start automatic cache refresh process
	stopCacheRefresh := startAutomaticCacheRefresh(ctx, dependencies.PriceService, cfg.Business.SupportedPairs, cfg.Cache.TTL)
	dependencies.StopCacheRefresh = stopCacheRefresh // Add for graceful shutdown

	// 6. Configure router with dependencies and configuration
	appRouter := router.NewRouter(dependencies.PriceService, cfg.Business.SupportedPairs, cfg.RateLimit, cfg.Auth)
	handler := appRouter.GetHandler()

	// 7. Crear servidor HTTP
	httpServer := server.NewServer(handler, cfg.Server.Port)

	// 8. Configurar graceful shutdown
	setupGracefulShutdown(ctx, httpServer, dependencies, cfg.Server.ShutdownTimeout)

	// 9. Iniciar servidor (llamada bloqueante)
	logging.Info(ctx, "Starting HTTP server", logging.Fields{
		"port":             cfg.Server.Port,
		"shutdown_timeout": cfg.Server.ShutdownTimeout,
	})
	log.Fatal(httpServer.Start())
}

// Dependencies encapsulates all application dependencies
type Dependencies struct {
	Exchange         interfaces.Exchange
	Cache            interfaces.Cache
	PriceService     interfaces.PriceService
	Config           *config.Config
	StopCacheRefresh func() // To stop the automatic cache refresh process
}

// loadConfiguration loads and validates the application configuration
func loadConfiguration(ctx context.Context) (*config.Config, error) {
	logging.Info(ctx, "Loading application configuration", nil)

	// Load configuration for current environment
	loader := config.NewLoader()
	environment := config.GetEnvironment()

	cfg, err := loader.LoadForEnvironment(environment)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	validator := config.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return nil, err
	}

	logging.Info(ctx, "Configuration loaded and validated successfully", logging.Fields{
		"environment":     environment,
		"server_port":     cfg.Server.Port,
		"cache_backend":   cfg.Cache.Backend,
		"supported_pairs": len(cfg.Business.SupportedPairs),
		"rate_limit":      cfg.RateLimit.Enabled,
	})

	return cfg, nil
}

// initializeLogging configures the logging system based on configuration
func initializeLogging(ctx context.Context, logConfig config.LoggingConfig) {
	// Create enhanced logger configuration
	loggerConfig := logging.ConfigFromEnvironment("btc-ltp-service", AppVersion).
		WithLevel(logging.LogLevelFromString(logConfig.Level)).
		WithFormat(logging.LogFormatFromString(logConfig.Format))

	// Initialize global loggers
	if err := logging.InitializeGlobalLoggers(loggerConfig); err != nil {
		log.Fatalf("Failed to initialize logging system: %v", err)
	}

	logging.Info(ctx, "Logging system initialized", logging.Fields{
		"level":       logConfig.Level,
		"format":      logConfig.Format,
		"service":     loggerConfig.Service,
		"version":     loggerConfig.Version,
		"environment": loggerConfig.Environment,
	})
}

// initializeDependencies configures and initializes all application dependencies
func initializeDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	logging.Info(ctx, "Initializing application dependencies", nil)

	// 1. Exchange client - usar Mock en development mode, sino Fallback real
	var exchangeClient interfaces.Exchange
	if cfg.Development.MockMode || cfg.Development.DevMode {
		exchangeClient = exchange.NewMockExchange()
		logging.Info(ctx, "Mock exchange initialized for development", logging.Fields{
			"type":       "MockExchange",
			"mock_mode":  cfg.Development.MockMode,
			"dev_mode":   cfg.Development.DevMode,
			"debug_mode": cfg.Development.DebugMode,
		})
	} else {
		exchangeClient = exchange.NewFallbackExchange(cfg.Exchange.Kraken, cfg.Business.SupportedPairs)
		logging.Info(ctx, "Fallback exchange initialized", logging.Fields{
			"primary":          "WebSocket",
			"secondary":        "REST",
			"websocket_url":    cfg.Exchange.Kraken.WebSocketURL,
			"rest_url":         cfg.Exchange.Kraken.RestURL,
			"timeout_seconds":  cfg.Exchange.Kraken.Timeout.Seconds(),
			"fallback_timeout": cfg.Exchange.Kraken.FallbackTimeout.Seconds(),
			"max_retries":      cfg.Exchange.Kraken.MaxRetries,
		})
	}

	// 2. Cache with configuration
	appCache, err := createCacheWithConfig(ctx, cfg.Cache)
	if err != nil {
		return nil, err
	}

	// 3. Price service with configuration
	priceService := services.NewPriceServiceWithTTL(exchangeClient, appCache, cfg.Cache.TTL, cfg.Business.SupportedPairs)

	exchangeType := "FallbackExchange"
	if cfg.Development.MockMode || cfg.Development.DevMode {
		exchangeType = "MockExchange"
	}

	logging.Info(ctx, "Price service initialized", logging.Fields{
		"cache_ttl_seconds": cfg.Cache.TTL.Seconds(),
		"cache_prefix":      cfg.Business.CachePrefix,
		"exchange_type":     exchangeType,
	})

	logging.Info(ctx, "All dependencies initialized successfully", nil)
	return &Dependencies{
		Exchange:     exchangeClient,
		Cache:        appCache,
		PriceService: priceService,
		Config:       cfg,
	}, nil
}

// setupGracefulShutdown configures graceful shutdown for the server
func setupGracefulShutdown(ctx context.Context, httpServer *server.Server, deps *Dependencies, shutdownTimeout time.Duration) {
	// Channel to receive OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine to handle graceful shutdown
	go func() {
		// Wait for signal
		sig := <-quit
		logging.Info(ctx, "Received shutdown signal", logging.Fields{
			"signal":           sig.String(),
			"shutdown_timeout": shutdownTimeout,
		})

		// Create timeout context for shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Stop HTTP server
		if err := httpServer.Stop(shutdownCtx); err != nil {
			logging.ErrorWithError(ctx, "Error during graceful shutdown", err, nil)
			os.Exit(1)
		}

		// Stop automatic cache refresh process
		if deps.StopCacheRefresh != nil {
			deps.StopCacheRefresh()
			logging.Info(ctx, "Automatic cache refresh process stopped", nil)
		}

		// Close exchange connections
		if fallbackExchange, ok := deps.Exchange.(*exchange.FallbackExchange); ok {
			if err := fallbackExchange.Close(); err != nil {
				logging.Warn(ctx, "Error closing exchange connections", logging.Fields{
					"error": err.Error(),
				})
			} else {
				logging.Info(ctx, "Exchange connections closed successfully", nil)
			}
		}

		logging.Info(ctx, "Graceful shutdown completed successfully", nil)
		os.Exit(0)
	}()
}

// createCacheWithConfig creates a cache instance based on configuration
func createCacheWithConfig(ctx context.Context, cacheConfig config.CacheConfig) (interfaces.Cache, error) {
	cacheFactory := cache.NewFactory()

	logging.Info(ctx, "Configuring cache", logging.Fields{
		"backend":            cacheConfig.Backend,
		"ttl_seconds":        cacheConfig.TTL.Seconds(),
		"redis_addr":         cacheConfig.Redis.Addr,
		"redis_db":           cacheConfig.Redis.DB,
		"redis_password_set": cacheConfig.Redis.Password != "",
	})

	return cacheFactory.CreateCacheFromEnv(
		cacheConfig.Backend,
		cacheConfig.Redis.Addr,
		cacheConfig.Redis.Password,
		cacheConfig.Redis.DB,
	)
}

// initializeCacheWithSupportedPairs pre-loads the cache with prices for all supported pairs
func initializeCacheWithSupportedPairs(ctx context.Context, priceService interfaces.PriceService, exch interfaces.Exchange, supportedPairs []string) error {
	if len(supportedPairs) == 0 {
		logging.Info(ctx, "No supported pairs configured, skipping cache initialization", nil)
		return nil
	}

	logging.Info(ctx, "Initializing cache with supported pairs", logging.Fields{
		"pairs_count": len(supportedPairs),
		"pairs":       supportedPairs,
	})

	// Create context with timeout to avoid long blocks at startup
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Pre-load all supported pairs in cache
	err := priceService.RefreshPrices(initCtx, supportedPairs)
	if err != nil {
		// Intentar warm-up rápido usando REST directamente si existe FallbackExchange
		if wu, ok := exch.(interfaces.WarmupExchange); ok {
			restCtx, cancelRest := context.WithTimeout(ctx, 15*time.Second)
			defer cancelRest()

			logging.Warn(ctx, "Warm-up via WebSocket failed, attempting WarmupExchange (REST)", logging.Fields{
				"pairs_count": len(supportedPairs),
			})

			if prices, restErr := wu.WarmupTickers(restCtx, supportedPairs); restErr == nil {
				// Convert prices to slice of pairs and cache via service
				_ = priceService.RefreshPrices(restCtx, supportedPairs)
				logging.Info(ctx, "Cache warm-up via WarmupExchange succeeded", logging.Fields{
					"retrieved_count": len(prices),
				})
				return nil
			}
		}

		logging.ErrorWithError(ctx, "Failed to initialize cache with supported pairs", err, logging.Fields{
			"pairs_count": len(supportedPairs),
			"pairs":       supportedPairs,
		})
		return err
	}

	logging.Info(ctx, "Successfully initialized cache with supported pairs", logging.Fields{
		"pairs_count": len(supportedPairs),
		"pairs":       supportedPairs,
	})
	return nil
}

// startAutomaticCacheRefresh starts an automatic process that updates the cache periodically
func startAutomaticCacheRefresh(ctx context.Context, priceService interfaces.PriceService, supportedPairs []string, refreshInterval time.Duration) func() {
	if len(supportedPairs) == 0 {
		logging.Info(ctx, "No supported pairs configured, skipping automatic cache refresh", nil)
		return func() {} // Return empty stop function
	}

	// Refresh BEFORE TTL expires to avoid data gaps and spikes
	// If TTL is 60s, use ~TTL/2 with minimum 30s
	interval := refreshInterval / 2
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}

	logging.Info(ctx, "Starting automatic cache refresh process", logging.Fields{
		"refresh_interval_seconds": interval.Seconds(),
		"pairs_count":              len(supportedPairs),
		"pairs":                    supportedPairs,
	})

	// Channel to stop the process
	stopChan := make(chan struct{})
	ticker := time.NewTicker(interval)

	// Goroutine that executes automatic updates
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Add small jitter (±10%) without external dependencies
				base := interval
				jitter := time.Duration(float64(base) * 0.1) // 10%
				// pseudo-random using current nanoseconds
				n := time.Now().UnixNano()
				delta := time.Duration(n%int64(2*jitter)) - jitter
				effectiveTimeout := 60*time.Second + delta/10 // slightly adjust timeout

				// Create context with timeout for each refresh
				refreshCtx, cancel := context.WithTimeout(context.Background(), effectiveTimeout)

				logging.Debug(refreshCtx, "Running automatic cache refresh", logging.Fields{
					"pairs_count": len(supportedPairs),
				})

				err := priceService.RefreshPrices(refreshCtx, supportedPairs)
				if err != nil {
					logging.Warn(refreshCtx, "Automatic cache refresh failed", logging.Fields{
						"error":       err.Error(),
						"pairs_count": len(supportedPairs),
						"pairs":       supportedPairs,
					})
				} else {
					logging.Debug(refreshCtx, "Automatic cache refresh completed successfully", logging.Fields{
						"pairs_count": len(supportedPairs),
					})
				}

				cancel()

			case <-stopChan:
				logging.Info(ctx, "Stopping automatic cache refresh process", nil)
				return
			}
		}
	}()

	// Return stop function
	return func() {
		close(stopChan)
	}
}
