// Package handlers provides HTTP request handlers for the dashboard API.
package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

// HealthResponse is the response for the health check endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
	Version   string `json:"version,omitempty"`
	GoVersion string `json:"go_version"`
}

// Health handles the health check endpoint.
func Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(startTime).Round(time.Second).String(),
		GoVersion: runtime.Version(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
