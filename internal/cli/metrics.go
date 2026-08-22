// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/observability"
)

var (
	metricsPort int
	metricsHost string
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Start a metrics server for monitoring",
	Long: `Start an HTTP server exposing Prometheus-compatible metrics.

The metrics server provides visibility into:
  - Release operations (total, successful, failed)
  - Plugin executions and errors
  - Command invocations and latency
  - Active release count

Example:
  # Start metrics server on default port 9090
  relicta metrics

  # Start on custom port
  relicta metrics --port 8080

  # Bind to specific interface
  relicta metrics --host 127.0.0.1 --port 9090

Metrics can be scraped by Prometheus or any compatible monitoring system.`,
	RunE: runMetrics,
}

func init() {
	rootCmd.AddCommand(metricsCmd)
	metricsCmd.Flags().IntVarP(&metricsPort, "port", "p", 9090, "Port to listen on")
	metricsCmd.Flags().StringVarP(&metricsHost, "host", "H", "0.0.0.0", "Host to bind to")
}

func runMetrics(cmd *cobra.Command, args []string) error {
	// telemetry.metrics supplies the defaults; a flag the operator typed wins.
	//
	// The whole telemetry block was read by nothing, so a repository that had configured a
	// port and a path got 9090 and /metrics regardless — and a Prometheus scraping the
	// configured address found nothing there.
	port, path := metricsPort, metricsPath()
	if !cmd.Flags().Changed("port") && cfg != nil && cfg.Telemetry.Metrics.Port > 0 {
		port = cfg.Telemetry.Metrics.Port
	}

	// Initialize global metrics with version
	metrics := observability.InitGlobal(versionInfo.Version)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle(path, metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Relicta Metrics</title></head>
<body>
<h1>Relicta Metrics Server</h1>
<p><a href="/metrics">Metrics</a> - Prometheus-compatible metrics endpoint</p>
<p><a href="/health">Health</a> - Health check endpoint</p>
</body>
</html>`))
	})

	addr := net.JoinHostPort(metricsHost, fmt.Sprintf("%d", port))
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("Starting metrics server on %s\n", addr)
		fmt.Printf("Metrics available at: http://%s/metrics\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down metrics server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// metricsPath is where the metrics are served, from telemetry.metrics.endpoint.
//
// Defaults to /metrics, which is what every Prometheus scrape config assumes and what this
// command served before the setting was read.
func metricsPath() string {
	if cfg == nil {
		return defaultMetricsPath
	}
	path := strings.TrimSpace(cfg.Telemetry.Metrics.Endpoint)
	if path == "" {
		return defaultMetricsPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// defaultMetricsPath is the conventional Prometheus path.
const defaultMetricsPath = "/metrics"
