// Package middleware provides HTTP middleware for the dashboard.
package middleware

import (
	"fmt"
	"net/http"
)

// SecurityHeaders returns middleware that sets security-related HTTP headers.
// These headers protect against common web vulnerabilities.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable XSS filter in browsers
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer policy for privacy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions policy (disable features not needed)
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// Content Security Policy
			// Allows self-hosted resources and inline styles (needed for Tailwind/Vue)
			// WebSocket connections allowed to same origin
			csp := "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: blob:; " +
				"font-src 'self'; " +
				"connect-src 'self' ws: wss:; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
			w.Header().Set("Content-Security-Policy", csp)

			next.ServeHTTP(w, r)
		})
	}
}

// StrictTransportSecurity returns middleware that sets HSTS header.
// Only enable this for HTTPS connections in production.
func StrictTransportSecurity(maxAge int) func(http.Handler) http.Handler {
	hstsHeader := fmt.Sprintf("max-age=%d; includeSubDomains", maxAge)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only set HSTS when using HTTPS
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				w.Header().Set("Strict-Transport-Security", hstsHeader)
			}
			next.ServeHTTP(w, r)
		})
	}
}
