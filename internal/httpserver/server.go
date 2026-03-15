// Package httpserver provides the HTTP server for the Relicta dashboard.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/httpserver/handlers"
	httpws "github.com/relicta-tech/relicta/internal/httpserver/websocket"
	"github.com/relicta-tech/relicta/internal/security/oidc"
	"github.com/relicta-tech/relicta/internal/security/token"
)

// Server is the HTTP server for the dashboard.
type Server struct {
	config       config.DashboardConfig
	router       chi.Router
	httpServer   *http.Server
	wsHub        *httpws.Hub
	sseHub       *handlers.SSEHub
	frontend     fs.FS
	tokenService *token.Service
	oidcHandlers *oidc.Handlers
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
		sseHub:   handlers.NewSSEHub(256),
		frontend: deps.Frontend,
	}

	// Create token service for session or OIDC authentication.
	if (deps.Config.Auth.Mode == config.DashboardAuthSession || deps.Config.Auth.Mode == config.DashboardAuthOIDC) &&
		deps.Config.Auth.SessionSecret != "" {
		ttl := deps.Config.Auth.SessionMaxAge
		if ttl == 0 {
			ttl = 24 * time.Hour
		}
		svc, err := token.NewService(token.Config{
			Secret:     []byte(deps.Config.Auth.SessionSecret),
			RefreshTTL: ttl,
		})
		if err == nil {
			s.tokenService = svc
		}
	}

	// Create OIDC service and handlers when OIDC auth mode is configured.
	if deps.Config.Auth.Mode == config.DashboardAuthOIDC && deps.Config.Auth.OIDC != nil && s.tokenService != nil {
		oidcCfg := *deps.Config.Auth.OIDC
		oidcCfg.Defaults()
		oidcSvc, err := oidc.NewService(context.Background(), oidcCfg)
		if err != nil {
			log.Printf("WARNING: OIDC provider discovery failed: %v", err)
		} else {
			s.oidcHandlers = oidc.NewHandlers(oidcSvc, s.tokenService)
		}
	}

	// Wire up WebSocket JWT authentication when a token service is available.
	if s.tokenService != nil {
		s.wsHub.SetTokenValidator(&tokenServiceAdapter{svc: s.tokenService})
	}

	// Set handler context for dependency injection
	handlers.SetContext(&handlers.Context{
		ReleaseServices: deps.ReleaseServices,
		TokenService:    s.tokenService,
		AuthConfig:      deps.Config.Auth,
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

	// Close SSE hub
	s.sseHub.Close()

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

// SSEHub returns the SSE hub for broadcasting events.
func (s *Server) SSEHub() *handlers.SSEHub {
	return s.sseHub
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

// tokenServiceAdapter adapts token.Service to the websocket.TokenValidator interface.
type tokenServiceAdapter struct {
	svc *token.Service
}

func (a *tokenServiceAdapter) Validate(tokenStr string) (string, []string, error) {
	claims, err := a.svc.Validate(tokenStr)
	if err != nil {
		return "", nil, err
	}
	return claims.Name, claims.Roles, nil
}
