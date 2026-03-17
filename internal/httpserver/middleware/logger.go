package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Logger returns a structured logging middleware.
func Logger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() { //nolint:contextcheck // Logging in defer uses request context captured at start
				duration := time.Since(start)
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				// Log level based on status code
				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}

				slog.Log(r.Context(), level, "http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", status,
					"duration_ms", duration.Milliseconds(),
					"bytes", ww.BytesWritten(),
					"request_id", chimw.GetReqID(r.Context()),
					"remote_addr", r.RemoteAddr,
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
