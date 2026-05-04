// Package vanta provides a Vanta REST API client for pushing Relicta
// governance evidence (audit chain entries, governance decisions, approval
// records, compliance reports) into Vanta as custom evidence artifacts.
//
// Vanta released its remote MCP server in April 2026 and a Claude plugin for
// developer-side workflows. Relicta complements Vanta: Relicta produces
// cryptographically attested release-decision evidence; Vanta ingests it as
// upstream provenance for SOC 2 / ISO 27001 / HIPAA / PCI control evidence.
//
// This package is intentionally minimal — it ships in the CLI binary and
// avoids a heavyweight plugin gRPC contract. A full standalone plugin can
// follow once the integration shape is validated with design partners.
package vanta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the default Vanta REST API base URL. Override via
// ClientConfig.BaseURL for sandbox tenants or self-hosted gateways.
const DefaultBaseURL = "https://api.vanta.com/v1"

// DefaultUserAgent identifies Relicta in Vanta API logs.
const DefaultUserAgent = "relicta-vanta-integration/1.0"

// DefaultTimeout caps individual HTTP requests.
const DefaultTimeout = 30 * time.Second

// ClientConfig holds runtime configuration for the Vanta client.
type ClientConfig struct {
	// APIToken is the Vanta API token (Bearer auth). Required.
	APIToken string

	// BaseURL overrides the default Vanta endpoint. Optional.
	BaseURL string

	// HTTPClient overrides the default *http.Client. Optional.
	// If nil, a client with DefaultTimeout is used.
	HTTPClient *http.Client

	// UserAgent sets the User-Agent header. Optional.
	UserAgent string
}

// Client is a Vanta REST API client.
type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

// NewClient constructs a Vanta API client. Returns an error if required
// configuration is missing.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, errors.New("vanta: APIToken is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	return &Client{cfg: cfg, httpClient: httpClient}, nil
}

// PushEvidence uploads a single Evidence record to Vanta.
//
// Returns the Vanta-assigned evidence ID on success.
func (c *Client) PushEvidence(ctx context.Context, ev Evidence) (string, error) {
	if err := ev.Validate(); err != nil {
		return "", fmt.Errorf("vanta: invalid evidence: %w", err)
	}

	body, err := json.Marshal(ev)
	if err != nil {
		return "", fmt.Errorf("vanta: marshal evidence: %w", err)
	}

	url := c.cfg.BaseURL + "/custom-evidence"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("vanta: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vanta: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Vanta returned 2xx but unparseable body — surface the body for diagnostics.
		return "", fmt.Errorf("vanta: parse response: %w (body=%q)", err, string(respBody))
	}

	return result.ID, nil
}

// PushBatch uploads a batch of evidence records sequentially. On any failure
// the caller receives a slice of partial successes and the first error.
//
// Sequential rather than parallel by design — Vanta rate limits are
// per-tenant and bursting concurrent calls trips throttling. Throughput is
// not the goal; reliability is.
func (c *Client) PushBatch(ctx context.Context, evidence []Evidence) ([]string, error) {
	ids := make([]string, 0, len(evidence))
	for i, ev := range evidence {
		id, err := c.PushEvidence(ctx, ev)
		if err != nil {
			return ids, fmt.Errorf("vanta: push evidence %d/%d: %w", i+1, len(evidence), err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// APIError captures a non-2xx response from Vanta.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vanta API error: status %d body=%q", e.StatusCode, e.Body)
}
