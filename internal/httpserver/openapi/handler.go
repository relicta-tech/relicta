// Package openapi provides the OpenAPI 3.0 specification for the Relicta Dashboard API.
package openapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var spec []byte

// Handler serves the embedded OpenAPI specification as JSON.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(spec)
}
