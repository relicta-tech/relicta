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

// Spec returns the embedded specification bytes.
//
// Exported so a test outside this package can compare the documented paths against the
// routes the router actually serves. Without that comparison the specification drifts
// silently: it described 18 of the API's routes while the router served considerably
// more, and nothing reported the gap — a published contract that omits endpoints is
// worse than an obviously incomplete one, because a client trusts it.
func Spec() []byte { return spec }
