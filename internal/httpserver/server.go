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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/release"
	"github.com/relicta-tech/relicta/v5/internal/httpserver/handlers"
	httpws "github.com/relicta-tech/relicta/v5/internal/httpserver/websocket"
	"github.com/relicta-tech/relicta/v5/internal/security/oidc"
	"github.com/relicta-tech/relicta/v5/internal/security/token"
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
	metrics      http.Handler
	metricsPath  string
	startupErr   error
}

// ServerDeps contains dependencies for creating a new server.
type ServerDeps struct {
	Config           config.DashboardConfig
	Frontend         fs.FS                 // Embedded frontend files (nil for API-only mode)
	ReleaseServices  *release.Services     // Release domain services (optional)
	MnemosEnabled    bool                  // Whether the Mnemos adapter was initialized.
	ChronosEnabled   bool                  // Whether the Chronos adapter was initialized.
	TokenRevocations token.RevocationStore // Durable session revocations (optional for embedders).
	// Metrics, when telemetry.metrics.enabled is set, is served at MetricsPath.
	//
	// A separate `relicta metrics` process was the only way to expose them, which is a
	// second port to open and a second thing to run beside a dashboard that is already
	// listening. telemetry.metrics.enabled had no reader at all.
	Metrics     http.Handler
	MetricsPath string
	// Observability is the assembled observability subsystem, or nil when the repository
	// configured no providers.
	//
	// nil is meaningful and must stay nil: the handlers report `not_configured` for it,
	// which is what distinguishes "nobody is watching" from "everything is healthy". They
	// used to receive nil always, because nothing ever called SetObservabilityService.
	Observability handlers.ObservabilityService
}

// NewServer creates a new HTTP server for the dashboard.
func NewServer(deps ServerDeps) *Server {
	s := &Server{
		config:   deps.Config,
		wsHub:    httpws.NewHub(deps.Config.CORSOrigins),
		sseHub:   handlers.NewSSEHub(256),
		frontend: deps.Frontend,
	}

	// Install the observability subsystem, when there is one. Package-level state on the
	// handlers, which is what their SetObservabilityService takes; nil leaves them reporting
	// that nothing is configured.
	if deps.Observability != nil {
		handlers.SetObservabilityService(deps.Observability)
	}

	s.metrics, s.metricsPath = deps.Metrics, deps.MetricsPath

	// Create token service for session or OIDC authentication. If authentication
	// is configured but cannot be initialized, keep the server from starting in a
	// state where login is unavailable or every protected request is rejected.
	if deps.Config.Auth.Mode == config.DashboardAuthAPIKey {
		hasUsableAPIKey := false
		for _, key := range deps.Config.Auth.APIKeys {
			if strings.TrimSpace(key.Key) != "" {
				hasUsableAPIKey = true
				break
			}
		}
		if !hasUsableAPIKey {
			s.startupErr = errors.New("dashboard auth mode api_key requires at least one non-empty dashboard.auth.api_keys[].key")
		}
	}

	needsTokenService := deps.Config.Auth.Mode == config.DashboardAuthSession ||
		deps.Config.Auth.Mode == config.DashboardAuthOIDC
	if needsTokenService && deps.Config.Auth.SessionSecret == "" {
		s.startupErr = errors.New("dashboard auth mode requires dashboard.auth.session_secret")
	}
	if needsTokenService && deps.Config.Auth.SessionSecret != "" {
		ttl := deps.Config.Auth.SessionMaxAge
		if ttl == 0 {
			ttl = 24 * time.Hour
		}
		svc, err := token.NewService(token.Config{
			Secret:      []byte(deps.Config.Auth.SessionSecret),
			RefreshTTL:  ttl,
			Revocations: deps.TokenRevocations,
		})
		if err == nil {
			s.tokenService = svc
		} else {
			s.startupErr = fmt.Errorf("initialize dashboard session tokens: %w", err)
		}
	}

	// Create OIDC service and handlers when OIDC auth mode is configured.
	if deps.Config.Auth.Mode == config.DashboardAuthOIDC {
		if deps.Config.Auth.OIDC == nil {
			s.startupErr = errors.New("dashboard auth mode oidc requires dashboard.auth.oidc configuration")
		} else if s.tokenService == nil {
			// Preserve the more specific token-service initialization error above.
			if s.startupErr == nil {
				s.startupErr = errors.New("OIDC authentication requires a valid dashboard session token service")
			}
		} else {
			oidcCfg := *deps.Config.Auth.OIDC
			oidcCfg.Defaults()
			discoveryCtx, cancelDiscovery := context.WithTimeout(context.Background(), 10*time.Second)
			oidcSvc, err := oidc.NewService(discoveryCtx, oidcCfg)
			cancelDiscovery()
			if err != nil {
				s.startupErr = fmt.Errorf("initialize OIDC authentication: %w", err)
				log.Printf("ERROR: OIDC provider discovery failed; dashboard will refuse to start: %v", err)
			} else {
				s.oidcHandlers = oidc.NewHandlers(oidcSvc, s.tokenService)
			}
		}
	}

	// The ticket endpoint authenticates using the same middleware as the REST API;
	// upgrades then consume its short-lived, single-use ticket.
	s.wsHub.RequireTickets(true)

	// Set handler context for dependency injection
	handlers.SetContext(&handlers.Context{
		ReleaseServices: deps.ReleaseServices,
		TokenService:    s.tokenService,
		AuthConfig:      deps.Config.Auth,
		MnemosEnabled:   deps.MnemosEnabled,
		ChronosEnabled:  deps.ChronosEnabled,
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
	if s.tokenService != nil {
		defer s.tokenService.Close()
	}
	if s.startupErr != nil {
		return fmt.Errorf("dashboard authentication is not ready: %w", s.startupErr)
	}
	// Refuse to expose an unauthenticated dashboard off-host. mode=none
	// grants anonymous (read-only) access; binding it to a non-loopback
	// address without an explicit opt-in would silently publish the
	// dashboard to the network.
	if err := guardInsecureExposure(s.config); err != nil {
		return err
	}

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

// guardInsecureExposure blocks startup when the dashboard would be both
// unauthenticated and reachable off-host. mode=none on a loopback address is
// fine for local development; binding it elsewhere requires the explicit
// auth.allow_insecure_no_auth opt-in.
func guardInsecureExposure(cfg config.DashboardConfig) error {
	mode := cfg.Auth.Mode
	if mode != config.DashboardAuthNone && mode != "" {
		return nil // authenticated
	}
	if cfg.Auth.AllowInsecureNoAuth {
		return nil // operator explicitly accepted the risk
	}
	if isLoopbackBind(cfg.Address) {
		return nil // not reachable off-host
	}
	return fmt.Errorf(
		"refusing to start dashboard with auth mode=none on non-loopback address %q: "+
			"configure dashboard.auth (api_key/session/oidc), bind to localhost, "+
			"or set dashboard.auth.allow_insecure_no_auth=true to override",
		cfg.Address)
}

// isLoopbackBind reports whether the bind address only serves the local host.
func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.TrimPrefix(addr, ":") // tolerate ":8080" form... falls through
		if host == addr {
			return false
		}
		host = ""
	}
	switch host {
	case "localhost":
		return true
	case "":
		// ":8080" / empty host binds all interfaces — not loopback-only.
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Close WebSocket hub
	s.wsHub.Close()

	// Close SSE hub
	s.sseHub.Close()
	if s.tokenService != nil {
		s.tokenService.Close()
	}

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
