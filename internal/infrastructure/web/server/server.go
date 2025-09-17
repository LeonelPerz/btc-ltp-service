package server

import (
	"btc-ltp-service/internal/infrastructure/logging"
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server encapsulates HTTP server configuration
type Server struct {
	httpServer *http.Server
	port       int
}

// NewServer creates a new server instance
func NewServer(handler http.Handler, port int) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		port: port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	ctx := context.Background()

	logging.Info(ctx, "HTTP server starting", logging.Fields{
		"port": s.port,
	})

	logging.Info(ctx, "Available endpoints", logging.Fields{
		"endpoints": []string{
			fmt.Sprintf("GET  http://localhost:%d/health", s.port),
			fmt.Sprintf("GET  http://localhost:%d/ready", s.port),
			fmt.Sprintf("GET  http://localhost:%d/api/v1/ltp?pair=BTC/USD", s.port),
			fmt.Sprintf("GET  http://localhost:%d/api/v1/ltp?pair=BTC/USD,ETH/USD", s.port),
			fmt.Sprintf("GET  http://localhost:%d/api/v1/ltp/cached", s.port),
			fmt.Sprintf("POST http://localhost:%d/api/v1/ltp/refresh?pairs=BTC/USD", s.port),
		},
	})

	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop(ctx context.Context) error {
	logging.Info(ctx, "Stopping HTTP server gracefully", logging.Fields{
		"port": s.port,
	})

	return s.httpServer.Shutdown(ctx)
}

// GetPort returns the configured port
func (s *Server) GetPort() int {
	return s.port
}
