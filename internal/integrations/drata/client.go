// Package drata provides a Drata REST API client for pushing Relicta
// governance evidence as compliance evidence artifacts.
//
// Drata's API focuses on extracting compliance data and programmatically
// uploading specific evidence types. Relicta complements Drata by providing
// upstream cryptographically attested release-decision evidence Drata cannot
// generate from CI/Git data alone.
//
// This package mirrors the Vanta integration shape so adopters running both
// platforms (or evaluating one against the other) can switch with minimal
// configuration churn. CLI subcommand: `relicta integrations drata push`.
package drata

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

// DefaultBaseURL is the default Drata REST API base URL.
const DefaultBaseURL = "https://api.drata.com/public-api/v1"

// DefaultUserAgent identifies Relicta in Drata API logs.
const DefaultUserAgent = "relicta-drata-integration/1.0"

// DefaultTimeout caps individual HTTP requests.
const DefaultTimeout = 30 * time.Second

// ClientConfig holds runtime configuration for the Drata client.
type ClientConfig struct {
	// APIToken is the Drata API token (Bearer auth). Required.
	APIToken string

	// WorkspaceID identifies the target Drata workspace. Optional —
	// some endpoints accept it via header rather than path.
	WorkspaceID string

	// BaseURL overrides the default Drata endpoint. Optional.
	BaseURL string

	// HTTPClient overrides the default *http.Client. Optional.
	HTTPClient *http.Client

	// UserAgent sets the User-Agent header. Optional.
	UserAgent string
}

// Client is a Drata REST API client.
type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

// NewClient constructs a Drata API client. Returns an error if required
// configuration is missing.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, errors.New("drata: APIToken is required")
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

// PushEvidence uploads a single Evidence record to Drata.
//
// Returns the Drata-assigned evidence ID on success.
func (c *Client) PushEvidence(ctx context.Context, ev Evidence) (string, error) {
	if err := ev.Validate(); err != nil {
		return "", fmt.Errorf("drata: invalid evidence: %w", err)
	}

	body, err := json.Marshal(ev)
	if err != nil {
		return "", fmt.Errorf("drata: marshal evidence: %w", err)
	}

	url := c.cfg.BaseURL + "/evidence"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("drata: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if c.cfg.WorkspaceID != "" {
		req.Header.Set("X-Drata-Workspace", c.cfg.WorkspaceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("drata: do request: %w", err)
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
		return "", fmt.Errorf("drata: parse response: %w (body=%q)", err, string(respBody))
	}

	return result.ID, nil
}

// PushBatch uploads a batch of evidence records sequentially.
func (c *Client) PushBatch(ctx context.Context, evidence []Evidence) ([]string, error) {
	ids := make([]string, 0, len(evidence))
	for i, ev := range evidence {
		id, err := c.PushEvidence(ctx, ev)
		if err != nil {
			return ids, fmt.Errorf("drata: push evidence %d/%d: %w", i+1, len(evidence), err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// APIError captures a non-2xx response from Drata.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("drata API error: status %d body=%q", e.StatusCode, e.Body)
}
