// Package httpserver provides the HTTP server for the Relicta dashboard.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/httpserver/handlers"
	httpws "github.com/relicta-tech/relicta/internal/httpserver/websocket"
)

// Server is the HTTP server for the dashboard.
type Server struct {
	config     config.DashboardConfig
	router     chi.Router
	httpServer *http.Server
	wsHub      *httpws.Hub
	frontend   fs.FS
}

// ServerDeps contains dependencies for creating a new server.
type ServerDeps struct {
	Config          config.DashboardConfig
	Frontend        fs.FS             // Embedded frontend files (nil for API-only mode)
	ReleaseServices *release.Services // Release domain services (optional)
}

// NewServer creates a new HTTP server for the dashboard.
func NewServer(deps ServerDeps) *Server {
	s := &Server{
		config:   deps.Config,
		wsHub:    httpws.NewHub(deps.Config.CORSOrigins),
		frontend: deps.Frontend,
	}

	// Set handler context for dependency injection
	handlers.SetContext(&handlers.Context{
		ReleaseServices: deps.ReleaseServices,
	})

	s.router = s.setupRouter()

	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.router,
		ReadTimeout:  s.getReadTimeout(),
		WriteTimeout: s.getWriteTimeout(),
		IdleTimeout:  s.getIdleTimeout(),
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	// Start WebSocket hub
	go s.wsHub.Run(ctx)

	// Start HTTP server
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Address, err)
	}

	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// Use a new context for shutdown since the original is canceled
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx) //nolint:contextcheck // Intentionally new context for graceful shutdown
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Close WebSocket hub
	s.wsHub.Close()

	// Shutdown HTTP server
	return s.httpServer.Shutdown(shutdownCtx)
}

// Address returns the server address.
func (s *Server) Address() string {
	return s.config.Address
}

// Hub returns the WebSocket hub for broadcasting events.
func (s *Server) Hub() *httpws.Hub {
	return s.wsHub
}

// EventBroadcaster returns an EventPublisher that broadcasts domain events to WebSocket clients.
func (s *Server) EventBroadcaster() *httpws.EventBroadcaster {
	return httpws.NewEventBroadcaster(s.wsHub)
}

// getReadTimeout returns the read timeout with default.
func (s *Server) getReadTimeout() time.Duration {
	if s.config.ReadTimeout > 0 {
		return s.config.ReadTimeout
	}
	return 15 * time.Second
}

// getWriteTimeout returns the write timeout with default.
func (s *Server) getWriteTimeout() time.Duration {
	if s.config.WriteTimeout > 0 {
		return s.config.WriteTimeout
	}
	return 15 * time.Second
}

// getIdleTimeout returns the idle timeout with default.
func (s *Server) getIdleTimeout() time.Duration {
	if s.config.IdleTimeout > 0 {
		return s.config.IdleTimeout
	}
	return 60 * time.Second
}
