package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Mock Provider -------------------------------------------------------

type mockProvider struct {
	name        string
	metrics     []MetricSample
	alerts      []Alert
	metricsErr  error
	alertsErr   error
	healthErr   error
	healthCalls int
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) QueryMetrics(_ context.Context, _ MetricQuery) ([]MetricSample, error) {
	return m.metrics, m.metricsErr
}

func (m *mockProvider) QueryAlerts(_ context.Context, _ time.Duration) ([]Alert, error) {
	return m.alerts, m.alertsErr
}

func (m *mockProvider) HealthCheck(_ context.Context) error {
	m.healthCalls++
	return m.healthErr
}

// --- MetricQuery.Validate ------------------------------------------------

func TestMetricQuery_Validate(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		query   MetricQuery
		wantErr bool
	}{
		{
			name: "valid query",
			query: MetricQuery{
				MetricName: "http_requests_total",
				Start:      now.Add(-1 * time.Hour),
				End:        now,
				Step:       15 * time.Second,
			},
		},
		{
			name: "missing metric name",
			query: MetricQuery{
				Start: now.Add(-1 * time.Hour),
				End:   now,
				Step:  15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing start",
			query: MetricQuery{
				MetricName: "m",
				End:        now,
				Step:       15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing end",
			query: MetricQuery{
				MetricName: "m",
				Start:      now.Add(-1 * time.Hour),
				Step:       15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "end before start",
			query: MetricQuery{
				MetricName: "m",
				Start:      now,
				End:        now.Add(-1 * time.Hour),
				Step:       15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero step",
			query: MetricQuery{
				MetricName: "m",
				Start:      now.Add(-1 * time.Hour),
				End:        now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Registry ------------------------------------------------------------

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	p := &mockProvider{name: "test"}

	if err := reg.Register("prom", p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := reg.Get("prom")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != p {
		t.Error("Get() returned different provider instance")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewRegistry()
	p := &mockProvider{name: "test"}

	_ = reg.Register("prom", p)
	err := reg.Register("prom", p)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestRegistry_RegisterEmptyName(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register("", &mockProvider{name: "test"})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegistry_RegisterNilProvider(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register("nil", nil)
	if err == nil {
		t.Error("expected error for nil provider")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("missing")
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register("a", &mockProvider{name: "a"})
	_ = reg.Register("b", &mockProvider{name: "b"})

	names := reg.List()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}
}

func TestRegistry_HealthCheckAll(t *testing.T) {
	reg := NewRegistry()

	healthy := &mockProvider{name: "ok"}
	unhealthy := &mockProvider{name: "fail", healthErr: fmt.Errorf("connection refused")}

	_ = reg.Register("ok", healthy)
	_ = reg.Register("fail", unhealthy)

	statuses := reg.HealthCheckAll(context.Background())
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	healthyCount := 0
	for _, s := range statuses {
		if s.Healthy {
			healthyCount++
		}
	}
	if healthyCount != 1 {
		t.Errorf("expected 1 healthy provider, got %d", healthyCount)
	}
}

// --- Prometheus Provider -------------------------------------------------

func TestPrometheusProvider_QueryMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("expected query parameter")
		}

		resp := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result": []map[string]any{
					{
						"metric": map[string]string{"__name__": "http_requests_total", "job": "api"},
						"values": [][]any{
							{1700000000.0, "42"},
							{1700000015.0, "43"},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL)

	now := time.Now()
	samples, err := p.QueryMetrics(context.Background(), MetricQuery{
		MetricName: "http_requests_total",
		Labels:     map[string]string{"job": "api"},
		Start:      now.Add(-1 * time.Hour),
		End:        now,
		Step:       15 * time.Second,
	})

	if err != nil {
		t.Fatalf("QueryMetrics() error = %v", err)
	}
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}
	if samples[0].Value != 42 {
		t.Errorf("expected value 42, got %f", samples[0].Value)
	}
}

func TestPrometheusProvider_QueryMetrics_InvalidQuery(t *testing.T) {
	p := NewPrometheusProvider("http://localhost:9090")
	_, err := p.QueryMetrics(context.Background(), MetricQuery{})
	if err == nil {
		t.Error("expected validation error for empty query")
	}
}

func TestPrometheusProvider_QueryMetrics_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL)
	now := time.Now()
	_, err := p.QueryMetrics(context.Background(), MetricQuery{
		MetricName: "m",
		Start:      now.Add(-1 * time.Hour),
		End:        now,
		Step:       15 * time.Second,
	})
	if err == nil {
		t.Error("expected error for server 500")
	}
}

func TestPrometheusProvider_QueryAlerts(t *testing.T) {
	now := time.Now()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/alerts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := map[string]any{
			"status": "success",
			"data": map[string]any{
				"alerts": []map[string]any{
					{
						"labels":      map[string]string{"alertname": "HighErrorRate", "severity": "critical"},
						"annotations": map[string]string{"summary": "Error rate is high"},
						"state":       "firing",
						"activeAt":    now.Add(-5 * time.Minute).Format(time.RFC3339),
					},
					{
						"labels":   map[string]string{"alertname": "OldAlert", "severity": "warning"},
						"state":    "firing",
						"activeAt": now.Add(-2 * time.Hour).Format(time.RFC3339),
					},
					{
						"labels": map[string]string{"alertname": "Resolved"},
						"state":  "inactive",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL)
	alerts, err := p.QueryAlerts(context.Background(), 1*time.Hour)
	if err != nil {
		t.Fatalf("QueryAlerts() error = %v", err)
	}

	// Should include HighErrorRate (within window), exclude OldAlert (outside window) and Resolved (inactive).
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].Name != "HighErrorRate" {
		t.Errorf("expected HighErrorRate, got %s", alerts[0].Name)
	}
	if alerts[0].Severity != "critical" {
		t.Errorf("expected critical severity, got %s", alerts[0].Severity)
	}
}

func TestPrometheusProvider_HealthCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/-/healthy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Prometheus Server is Healthy."))
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL)
	err := p.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

func TestPrometheusProvider_HealthCheck_Unhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL)
	err := p.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for unhealthy server")
	}
}

func TestPrometheusProvider_BasicAuth(t *testing.T) {
	var gotUser, gotPass string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL, WithBasicAuth("admin", "secret"))
	_ = p.HealthCheck(context.Background())

	if gotUser != "admin" || gotPass != "secret" {
		t.Errorf("expected basic auth admin:secret, got %s:%s", gotUser, gotPass)
	}
}

func TestPrometheusProvider_BearerToken(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	p := NewPrometheusProvider(ts.URL, WithBearerToken("my-token"))
	_ = p.HealthCheck(context.Background())

	if gotAuth != "Bearer my-token" {
		t.Errorf("expected bearer token header, got %q", gotAuth)
	}
}

func TestBuildPromQL(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		labels     map[string]string
		wantExact  bool
		want       string
	}{
		{
			name:       "no labels",
			metricName: "up",
			want:       "up",
			wantExact:  true,
		},
		{
			name:       "with labels",
			metricName: "http_requests_total",
			labels:     map[string]string{"job": "api"},
			want:       `http_requests_total{job="api"}`,
			wantExact:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPromQL(tt.metricName, tt.labels)
			if tt.wantExact && got != tt.want {
				t.Errorf("buildPromQL() = %q, want %q", got, tt.want)
			}
		})
	}
}
