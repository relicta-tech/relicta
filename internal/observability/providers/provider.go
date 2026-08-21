// Package providers defines interfaces and a registry for querying external
// observability systems (Prometheus, Datadog, etc.) from within Relicta.
package providers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Provider is the interface for querying an external observability backend.
type Provider interface {
	// Name returns the human-readable provider name.
	Name() string

	// QueryMetrics executes a metric query and returns time-series samples.
	QueryMetrics(ctx context.Context, query MetricQuery) ([]MetricSample, error)

	// QueryAlerts returns currently firing alerts within the given window.
	QueryAlerts(ctx context.Context, window time.Duration) ([]Alert, error)

	// HealthCheck verifies that the provider endpoint is reachable.
	HealthCheck(ctx context.Context) error
}

// MetricQuery describes a time-series query to run against a provider.
type MetricQuery struct {
	// MetricName is the metric to query (e.g., "http_requests_total").
	MetricName string `json:"metric_name"`
	// Labels filters the metric by label matchers.
	Labels map[string]string `json:"labels,omitempty"`
	// Start is the beginning of the query range.
	Start time.Time `json:"start"`
	// End is the end of the query range.
	End time.Time `json:"end"`
	// Step is the resolution step for range queries.
	Step time.Duration `json:"step"`
}

// Validate checks that the query has the minimum required fields.
func (q MetricQuery) Validate() error {
	if q.MetricName == "" {
		return fmt.Errorf("metric name is required")
	}
	if q.Start.IsZero() {
		return fmt.Errorf("start time is required")
	}
	if q.End.IsZero() {
		return fmt.Errorf("end time is required")
	}
	if !q.End.After(q.Start) {
		return fmt.Errorf("end time must be after start time")
	}
	if q.Step <= 0 {
		return fmt.Errorf("step must be positive")
	}
	return nil
}

// MetricSample is a single data point returned by a provider query.
type MetricSample struct {
	// Timestamp is when the sample was recorded.
	Timestamp time.Time `json:"timestamp"`
	// Value is the metric value at this timestamp.
	Value float64 `json:"value"`
	// Labels are the metric labels associated with this sample.
	Labels map[string]string `json:"labels,omitempty"`
}

// Alert represents an alert from an external observability system.
type Alert struct {
	// Name is the alert rule name.
	Name string `json:"name"`
	// Severity indicates the alert severity (critical, warning, info).
	Severity string `json:"severity"`
	// StartedAt is when the alert started firing.
	StartedAt time.Time `json:"started_at"`
	// Labels are alert labels for routing and identification.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are human-readable alert annotations.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ProviderConfig holds provider connection configuration.
type ProviderConfig struct {
	// Type is the provider type (e.g., "prometheus").
	Type string `json:"type"`
	// Endpoint is the base URL of the provider API.
	Endpoint string `json:"endpoint"`
	// BasicAuthUser is the username for basic auth (optional).
	BasicAuthUser string `json:"basic_auth_user,omitempty"`
	// BasicAuthPass is the password for basic auth (optional).
	BasicAuthPass string `json:"basic_auth_pass,omitempty"`
	// BearerToken is a bearer token for authentication (optional).
	BearerToken string `json:"bearer_token,omitempty"`
}

// ProviderStatus reports the current status of a configured provider.
type ProviderStatus struct {
	// Name is the provider name.
	Name string `json:"name"`
	// Type is the provider type.
	Type string `json:"type"`
	// Endpoint is the configured endpoint.
	Endpoint string `json:"endpoint"`
	// Healthy indicates whether the last health check succeeded.
	Healthy bool `json:"healthy"`
	// LastChecked is when the provider was last health-checked.
	LastChecked *time.Time `json:"last_checked,omitempty"`
	// Error contains the last error message if unhealthy.
	Error string `json:"error,omitempty"`
}

// Registry maintains a set of named provider instances.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider under the given name.
// Returns an error if the name is already registered.
func (r *Registry) Register(name string, p Provider) error {
	if name == "" {
		return fmt.Errorf("provider name is required")
	}
	if p == nil {
		return fmt.Errorf("provider is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider already registered: %s", name)
	}
	r.providers[name] = p
	return nil
}

// Get returns the provider registered under the given name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}

// List returns the names of all registered providers.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// HealthCheckAll runs a health check against every registered provider
// and returns their statuses.
func (r *Registry) HealthCheckAll(ctx context.Context) []ProviderStatus {
	r.mu.RLock()
	providers := make(map[string]Provider, len(r.providers))
	for k, v := range r.providers {
		providers[k] = v
	}
	r.mu.RUnlock()

	statuses := make([]ProviderStatus, 0, len(providers))
	now := time.Now()

	for name, p := range providers {
		status := ProviderStatus{
			Name:        name,
			Type:        p.Name(),
			LastChecked: &now,
		}

		// A status about connectivity that does not say where it tried to connect is most of
		// an answer. Optional, so a provider that has no single endpoint simply omits it.
		if addressable, ok := p.(interface{ Endpoint() string }); ok {
			status.Endpoint = addressable.Endpoint()
		}

		if err := p.HealthCheck(ctx); err != nil {
			status.Healthy = false
			status.Error = err.Error()
		} else {
			status.Healthy = true
		}

		statuses = append(statuses, status)
	}

	return statuses
}
