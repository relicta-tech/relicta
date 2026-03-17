package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PrometheusProvider implements Provider using the Prometheus HTTP API.
type PrometheusProvider struct {
	endpoint string
	client   *http.Client
	auth     promAuth
}

type promAuth struct {
	basicUser string
	basicPass string
	bearerTok string
}

// NewPrometheusProvider creates a provider that queries a Prometheus-compatible API.
func NewPrometheusProvider(endpoint string, opts ...PrometheusOption) *PrometheusProvider {
	p := &PrometheusProvider{
		endpoint: strings.TrimRight(endpoint, "/"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// PrometheusOption configures a PrometheusProvider.
type PrometheusOption func(*PrometheusProvider)

// WithBasicAuth sets basic authentication credentials.
func WithBasicAuth(user, pass string) PrometheusOption {
	return func(p *PrometheusProvider) {
		p.auth.basicUser = user
		p.auth.basicPass = pass
	}
}

// WithBearerToken sets bearer token authentication.
func WithBearerToken(token string) PrometheusOption {
	return func(p *PrometheusProvider) {
		p.auth.bearerTok = token
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(c *http.Client) PrometheusOption {
	return func(p *PrometheusProvider) {
		p.client = c
	}
}

// Name returns the provider type identifier.
func (p *PrometheusProvider) Name() string {
	return "prometheus"
}

// QueryMetrics implements Provider.QueryMetrics using /api/v1/query_range.
func (p *PrometheusProvider) QueryMetrics(ctx context.Context, query MetricQuery) ([]MetricSample, error) {
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Build PromQL expression from metric name and labels.
	expr := buildPromQL(query.MetricName, query.Labels)

	params := url.Values{}
	params.Set("query", expr)
	params.Set("start", strconv.FormatInt(query.Start.Unix(), 10))
	params.Set("end", strconv.FormatInt(query.End.Unix(), 10))
	params.Set("step", strconv.FormatInt(int64(query.Step.Seconds()), 10))

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", p.endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseQueryRangeResponse(body)
}

// QueryAlerts implements Provider.QueryAlerts using /api/v1/alerts.
func (p *PrometheusProvider) QueryAlerts(ctx context.Context, window time.Duration) ([]Alert, error) {
	reqURL := fmt.Sprintf("%s/api/v1/alerts", p.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseAlertsResponse(body, window)
}

// HealthCheck implements Provider.HealthCheck.
func (p *PrometheusProvider) HealthCheck(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/-/healthy", p.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	p.applyAuth(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// applyAuth sets authentication headers on the request.
func (p *PrometheusProvider) applyAuth(req *http.Request) {
	if p.auth.bearerTok != "" {
		req.Header.Set("Authorization", "Bearer "+p.auth.bearerTok)
	} else if p.auth.basicUser != "" {
		req.SetBasicAuth(p.auth.basicUser, p.auth.basicPass)
	}
}

// buildPromQL constructs a simple PromQL selector from a metric name and labels.
func buildPromQL(metricName string, labels map[string]string) string {
	if len(labels) == 0 {
		return metricName
	}

	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return fmt.Sprintf("%s{%s}", metricName, strings.Join(parts, ","))
}

// Prometheus API response structures.

type promAPIResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error,omitempty"`
}

type promQueryRangeData struct {
	ResultType string             `json:"resultType"`
	Result     []promMatrixResult `json:"result"`
}

type promMatrixResult struct {
	Metric map[string]string `json:"metric"`
	Values []promValue       `json:"values"`
}

type promValue [2]json.RawMessage // [timestamp, value]

type promAlertsData struct {
	Alerts []promAlert `json:"alerts"`
}

type promAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    time.Time         `json:"activeAt"`
}

// parseQueryRangeResponse parses the Prometheus /api/v1/query_range response.
func parseQueryRangeResponse(body []byte) ([]MetricSample, error) {
	var apiResp promAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if apiResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", apiResp.Error)
	}

	var data promQueryRangeData
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}

	var samples []MetricSample
	for _, result := range data.Result {
		for _, v := range result.Values {
			ts, val, err := parsePromValue(v)
			if err != nil {
				continue
			}
			samples = append(samples, MetricSample{
				Timestamp: ts,
				Value:     val,
				Labels:    result.Metric,
			})
		}
	}

	return samples, nil
}

// parsePromValue extracts timestamp and float value from a Prometheus value tuple.
func parsePromValue(v promValue) (time.Time, float64, error) {
	var tsFloat float64
	if err := json.Unmarshal(v[0], &tsFloat); err != nil {
		return time.Time{}, 0, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	var valStr string
	if err := json.Unmarshal(v[1], &valStr); err != nil {
		return time.Time{}, 0, fmt.Errorf("failed to parse value string: %w", err)
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("failed to parse float value: %w", err)
	}

	ts := time.Unix(int64(tsFloat), int64((tsFloat-float64(int64(tsFloat)))*1e9))
	return ts, val, nil
}

// parseAlertsResponse parses the Prometheus /api/v1/alerts response.
func parseAlertsResponse(body []byte, window time.Duration) ([]Alert, error) {
	var apiResp promAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if apiResp.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", apiResp.Error)
	}

	var data promAlertsData
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse alerts data: %w", err)
	}

	cutoff := time.Now().Add(-window)
	var alerts []Alert
	for _, a := range data.Alerts {
		if a.State != "firing" {
			continue
		}
		if !a.ActiveAt.IsZero() && a.ActiveAt.Before(cutoff) {
			continue
		}
		severity := a.Labels["severity"]
		if severity == "" {
			severity = "warning"
		}
		alerts = append(alerts, Alert{
			Name:        a.Labels["alertname"],
			Severity:    severity,
			StartedAt:   a.ActiveAt,
			Labels:      a.Labels,
			Annotations: a.Annotations,
		})
	}

	return alerts, nil
}

// Compile-time interface check.
var _ Provider = (*PrometheusProvider)(nil)
