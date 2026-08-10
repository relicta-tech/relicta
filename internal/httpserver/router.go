package httpserver

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/relicta-tech/relicta/v4/internal/httpserver/handlers"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/middleware"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/openapi"
)

// setupRouter configures the Chi router with all routes and middleware.
func (s *Server) setupRouter() chi.Router {
	r := chi.NewRouter()

	// Core middleware
	// Note: RealIP runs after rate limiting so that rate limits use the raw
	// TCP peer address and cannot be bypassed via X-Forwarded-For spoofing.
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger())
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))

	// Security headers
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.StrictTransportSecurity(63072000)) // 2 years

	// CORS configuration
	r.Use(s.corsMiddleware())

	// Health check (unauthenticated, outside API version negotiation)
	r.Get("/health", handlers.Health)

	// Kubernetes-style probes (unauthenticated)
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz)
	r.Get("/health/cognitive", handlers.CognitiveHealth)

	// All /api/v1 routes share API version negotiation
	r.Route("/api/v1", func(r chi.Router) {
		// chimw.RealIP was removed: it trusts X-Forwarded-For / X-Real-IP
		// unconditionally (GHSA-3fxj-6jh8-hvhx), letting clients spoof their
		// IP for rate limiting and audit logs. Direct peer address is the
		// secure default; deployments behind a trusted proxy should resolve
		// client IPs at the proxy layer.
		r.Use(middleware.APIVersion())

		// Health check (unauthenticated)
		r.Get("/health", handlers.Health)

		// OpenAPI specification (unauthenticated)
		r.Get("/openapi.json", openapi.Handler)

		// Auth endpoints (unauthenticated, rate-limited)
		r.Route("/auth", func(r chi.Router) {
			r.Use(middleware.RateLimit(&middleware.RateLimiterConfig{
				Rate:     10,
				Burst:    5,
				Interval: time.Minute,
			}))
			r.Post("/login", handlers.Login)
			r.Post("/refresh", handlers.Refresh)
			r.Post("/logout", handlers.Logout)

			// OIDC SSO endpoints
			if s.oidcHandlers != nil {
				r.Get("/oidc/login", s.oidcHandlers.LoginRedirect)
				r.Get("/oidc/callback", s.oidcHandlers.Callback)
			}
		})

		// Authenticated API routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(s.config.Auth, s.tokenService))

			// WebSocket endpoint
			r.Get("/ws", s.handleWebSocket)

			// Server-Sent Events endpoint (WebSocket fallback)
			r.Get("/events/stream", handlers.SSEStreamHandler(s.sseHub))

			// Release endpoints
			r.Route("/releases", func(r chi.Router) {
				r.Get("/", handlers.ListReleases)
				r.Get("/active", handlers.GetActiveRelease)
				r.Get("/{id}", handlers.GetRelease)
				r.Get("/{id}/events", handlers.GetReleaseEvents)

				// The ADR-009 recommendation artifact for a run. ADR-009 names this
				// API as one of the three interfaces that return the artifact, and it
				// returned none — a Hub reading over HTTP got a different shape than an
				// agent reading MCP for the same release.
				r.Get("/{id}/recommendation", handlers.GetReleaseRecommendation)
			})

			// Governance endpoints
			r.Route("/governance", func(r chi.Router) {
				r.Get("/decisions", handlers.ListGovernanceDecisions)
				r.Get("/risk-trends", handlers.GetRiskTrends)
				r.Get("/factors", handlers.GetFactorDistribution)
			})

			// Actor endpoints
			r.Route("/actors", func(r chi.Router) {
				r.Get("/", handlers.ListActors)
				r.Get("/{id}", handlers.GetActor)
			})

			// Approval endpoints
			r.Route("/approvals", func(r chi.Router) {
				r.Get("/pending", handlers.ListPendingApprovals)
				r.Post("/{id}/approve", handlers.ApproveRelease)
				r.Post("/{id}/reject", handlers.RejectRelease)
			})

			// Analytics endpoints
			r.Route("/analytics", func(r chi.Router) {
				r.Get("/risk-trends", handlers.GetAnalyticsRiskTrends)
				r.Get("/decisions", handlers.GetAnalyticsDecisions)
				r.Get("/team", handlers.GetAnalyticsTeam)
				r.Get("/outcomes", handlers.GetAnalyticsOutcomes)
				r.Get("/risk-factors", handlers.GetAnalyticsRiskFactors)
				r.Get("/calibration", handlers.GetAnalyticsCalibration)
			})

			// Memory/Learning endpoints
			r.Route("/memory", func(r chi.Router) {
				r.Get("/insights", handlers.GetMemoryInsights)
				r.Get("/trends", handlers.GetMemoryTrends)
			})

			// Audit trail endpoint
			r.Get("/audit", handlers.ListAuditEvents)

			// Webhook delivery endpoints
			r.Route("/webhooks", func(r chi.Router) {
				r.Get("/{id}/deliveries", handlers.ListWebhookDeliveries)
				r.Post("/{id}/deliveries/{deliveryId}/redeliver", handlers.RedeliverWebhook)
			})

			// Multi-repository governance endpoints
			r.Route("/groups", func(r chi.Router) {
				r.Get("/", handlers.ListGroups)
				r.Get("/{name}/status", handlers.GetGroupStatus)
				r.Get("/{name}/graph", handlers.GetGroupGraph)
			})

			// Observability endpoints
			r.Route("/observability", func(r chi.Router) {
				r.Get("/health", handlers.GetObservabilityHealth)
				r.Get("/correlations", handlers.GetObservabilityCorrelations)
				r.Get("/providers", handlers.GetObservabilityProviders)
				r.Post("/webhook/{provider}", handlers.ObservabilityWebhook)
			})
		})
	})

	// Serve frontend static files
	if s.frontend != nil {
		s.serveFrontend(r)
	}

	return r
}

// corsMiddleware returns configured CORS middleware.
func (s *Server) corsMiddleware() func(http.Handler) http.Handler {
	allowedOrigins := s.config.CORSOrigins
	if len(allowedOrigins) == 0 {
		// Default: same-origin only (no CORS headers sent)
		allowedOrigins = []string{}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-ID", "X-API-Version"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// handleWebSocket handles WebSocket upgrade requests.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.wsHub.HandleConnection(w, r)
}

// serveFrontend sets up static file serving for the embedded frontend.
func (s *Server) serveFrontend(r chi.Router) {
	// The frontend FS contains files directly at root (index.html, assets/)
	frontendFS := s.frontend

	// Read index.html once for SPA fallback
	indexHTML, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		// No index.html found - frontend not properly embedded
		return
	}

	// Create file server for static assets
	fileServer := http.FileServer(http.FS(frontendFS))

	// Serve root path explicitly
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	// Serve favicon
	r.Get("/favicon.svg", func(w http.ResponseWriter, req *http.Request) {
		fileServer.ServeHTTP(w, req)
	})
	r.Get("/favicon.ico", func(w http.ResponseWriter, req *http.Request) {
		fileServer.ServeHTTP(w, req)
	})

	// Serve static assets (js, css, images, etc.)
	r.Get("/assets/*", func(w http.ResponseWriter, req *http.Request) {
		fileServer.ServeHTTP(w, req)
	})

	// SPA catch-all - serve index.html for client-side routing
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		// Don't serve index.html for API routes
		if len(req.URL.Path) >= 4 && req.URL.Path[:4] == "/api" {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
}
