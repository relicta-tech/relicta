package httpserver

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/relicta-tech/relicta/internal/httpserver/handlers"
	"github.com/relicta-tech/relicta/internal/httpserver/middleware"
)

// setupRouter configures the Chi router with all routes and middleware.
func (s *Server) setupRouter() chi.Router {
	r := chi.NewRouter()

	// Core middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger())
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))

	// CORS configuration
	r.Use(s.corsMiddleware())

	// Health check (unauthenticated)
	r.Get("/health", handlers.Health)
	r.Get("/api/v1/health", handlers.Health)

	// API routes (authenticated)
	r.Route("/api/v1", func(r chi.Router) {
		// Apply authentication middleware
		r.Use(middleware.Auth(s.config.Auth))

		// WebSocket endpoint
		r.Get("/ws", s.handleWebSocket)

		// Release endpoints
		r.Route("/releases", func(r chi.Router) {
			r.Get("/", handlers.ListReleases)
			r.Get("/active", handlers.GetActiveRelease)
			r.Get("/{id}", handlers.GetRelease)
			r.Get("/{id}/events", handlers.GetReleaseEvents)
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

		// Audit trail endpoint
		r.Get("/audit", handlers.ListAuditEvents)
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
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
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
		w.Write(indexHTML)
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
		w.Write(indexHTML)
	})
}
