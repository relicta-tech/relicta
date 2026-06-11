package hubsync

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

	"github.com/relicta-tech/relicta/internal/infrastructure/webhook/retry"
)

// DefaultUserAgent identifies the CLI in Hub access logs.
const DefaultUserAgent = "relicta-cli-hubsync/1.0"

// DefaultTimeout caps individual HTTP attempts. Multiple attempts are
// independently bounded; total wall time is roughly DefaultTimeout * (1+retries).
const DefaultTimeout = 30 * time.Second

// ClientConfig configures the Hub sync client.
type ClientConfig struct {
	// HubURL is the base URL of the Hub instance, e.g. "https://hub.example.com".
	// Required. /api/v1/sync is appended.
	HubURL string

	// AuthToken is the operator's Hub bearer token (JWT or API token).
	// Optional in alpha / unauthenticated dev mode; required against any
	// Hub instance that has JWT_SECRET configured.
	AuthToken string

	// HTTPClient overrides the default HTTP client. Optional.
	HTTPClient *http.Client

	// UserAgent overrides the default user-agent. Optional.
	UserAgent string

	// Retry overrides retry tuning. Optional; falls back to retry.DefaultConfig().
	Retry *retry.Config
}

// Client posts batches of governance events to a Hub /api/v1/sync endpoint.
type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
	retry      retry.Config
}

// NewClient validates config and returns a ready Client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.HubURL) == "" {
		return nil, errors.New("hubsync: HubURL is required")
	}
	cfg.HubURL = strings.TrimRight(cfg.HubURL, "/")
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	r := retry.DefaultConfig()
	if cfg.Retry != nil {
		r = *cfg.Retry
	}

	return &Client{cfg: cfg, httpClient: httpClient, retry: r}, nil
}

// Push sends a batch payload (the JSON body Hub's /sync expects) to Hub with
// exponential backoff. Returns the parsed response on success, or the last
// transport error wrapped in &TransportError{} if every attempt failed.
//
// Per-event semantics (rejected/failed entries inside a 207) are surfaced via
// the returned SyncResponse — the caller decides which to re-queue.
func (c *Client) Push(ctx context.Context, payload []byte) (*SyncResponse, error) {
	url := c.cfg.HubURL + "/api/v1/sync"

	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retry.NextDelay(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.attempt(ctx, url, payload)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Non-retryable error short-circuits the loop. 4xx (except 429) are
		// caller bugs — retrying won't help. Network errors and 5xx retry.
		if !shouldRetry(err) {
			return nil, err
		}
	}
	return nil, &TransportError{
		Attempts: c.retry.MaxRetries + 1,
		Cause:    lastErr,
	}
}

// attempt performs a single POST. Errors classified into retryable (network,
// 5xx, 429) vs terminal (4xx other than 429) so Push() can decide whether to
// continue.
func (c *Client) attempt(ctx context.Context, url string, payload []byte) (*SyncResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, &TransportError{Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CGP-Version", CGPVersion)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if c.cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &TransportError{Cause: err}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusMultiStatus:
		// 202 (all accepted) and 207 (partial) both decode the same body.
		var parsed SyncResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, &TransportError{
				StatusCode: resp.StatusCode,
				Cause:      fmt.Errorf("decode response: %w", err),
			}
		}
		return &parsed, nil

	case http.StatusPreconditionFailed:
		// Schema version mismatch — terminal: a CLI upgrade is the fix,
		// not a retry.
		return nil, &VersionMismatchError{Body: string(body)}

	case http.StatusServiceUnavailable:
		// Hub draining — operator pulled the instance from rotation. The
		// caller's queue should kick in and try again later.
		retryAfter := resp.Header.Get("Retry-After")
		return nil, &TransportError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(retryAfter),
			Cause:      fmt.Errorf("hub draining (Retry-After=%s)", retryAfter),
		}

	default:
		return nil, &TransportError{
			StatusCode: resp.StatusCode,
			Cause:      fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body)),
		}
	}
}

// shouldRetry classifies errors. Network errors (no StatusCode) and 5xx/429
// retry; 4xx other than 429 do not.
func shouldRetry(err error) bool {
	var verErr *VersionMismatchError
	if errors.As(err, &verErr) {
		return false
	}
	var trErr *TransportError
	if !errors.As(err, &trErr) {
		return true // unknown error type — be conservative
	}
	if trErr.StatusCode == 0 {
		return true // network error
	}
	if trErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return trErr.StatusCode >= 500
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	// RFC 7231 allows seconds-as-int OR HTTP-date. Alpha: int only.
	var secs int
	if _, err := fmt.Sscanf(h, "%d", &secs); err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// TransportError signals a sync failure. StatusCode is 0 for network-level
// failures (connection refused, DNS, timeout); otherwise the HTTP status.
type TransportError struct {
	StatusCode int
	RetryAfter time.Duration
	Attempts   int
	Cause      error
}

func (e *TransportError) Error() string {
	if e.Attempts > 0 {
		return fmt.Sprintf("hubsync: transport failure after %d attempt(s): %v", e.Attempts, e.Cause)
	}
	return fmt.Sprintf("hubsync: transport failure (status=%d): %v", e.StatusCode, e.Cause)
}

func (e *TransportError) Unwrap() error { return e.Cause }

// VersionMismatchError signals Hub rejected the wire-format version. The
// only fix is a CLI upgrade; retrying won't help.
type VersionMismatchError struct {
	Body string
}

func (e *VersionMismatchError) Error() string {
	return "hubsync: Hub rejected X-CGP-Version=" + CGPVersion + " (upgrade CLI): " + e.Body
}
