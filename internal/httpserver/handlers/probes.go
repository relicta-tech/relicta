package handlers

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// ComponentStatus represents the health of a single component.
type ComponentStatus struct {
	Status  string `json:"status"`            // "up" or "down"
	Message string `json:"message,omitempty"` // optional detail
}

// LivenessResponse is the response for the /healthz liveness probe.
type LivenessResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// ReadinessResponse is the response for the /readyz readiness probe.
type ReadinessResponse struct {
	Status     string                     `json:"status"` // "ready" or "not_ready"
	Timestamp  string                     `json:"timestamp"`
	Uptime     string                     `json:"uptime"`
	Version    string                     `json:"version,omitempty"`
	GoVersion  string                     `json:"go_version"`
	Components map[string]ComponentStatus `json:"components"`
}

// ReadinessChecker is a function that checks whether a component is ready.
type ReadinessChecker func() ComponentStatus

// ReadinessCheckers holds named readiness checkers that are evaluated by the
// readiness probe handler.
var ReadinessCheckers []struct {
	Name    string
	Checker ReadinessChecker
}

// RegisterReadinessChecker registers a named readiness checker.
func RegisterReadinessChecker(name string, checker ReadinessChecker) {
	ReadinessCheckers = append(ReadinessCheckers, struct {
		Name    string
		Checker ReadinessChecker
	}{Name: name, Checker: checker})
}

// Healthz handles the /healthz liveness probe.
// It always returns 200 as long as the server process is running.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	resp := LivenessResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Readyz handles the /readyz readiness probe.
// It evaluates all registered readiness checkers and returns 200 if all
// components are healthy, or 503 if any component is down.
func Readyz(w http.ResponseWriter, _ *http.Request) {
	components := make(map[string]ComponentStatus, len(ReadinessCheckers))
	allReady := true

	for _, rc := range ReadinessCheckers {
		status := rc.Checker()
		components[rc.Name] = status
		if status.Status != "up" {
			allReady = false
		}
	}

	overallStatus := "ready"
	httpStatus := http.StatusOK
	if !allReady {
		overallStatus = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := ReadinessResponse{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Uptime:     time.Since(startTime).Round(time.Second).String(),
		GoVersion:  runtime.Version(),
		Components: components,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}
