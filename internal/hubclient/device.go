package hubclient

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

// The client half of the device authorization grant (RFC 8628).
//
// The CLI asks Hub for a code, shows it to the person running the command, and polls until they
// approve it in a browser. Nothing is pasted, and the CLI never sees the password.

// DeviceAuthorization is what Hub returns when a flow starts.
type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Client talks to one Hub.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for a Hub base URL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		// A bounded per-request timeout. The flow as a whole runs for minutes, but a single
		// request that hangs should fail rather than stall the loop that is meant to be
		// polling — otherwise one dropped connection silently ends the wait.
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

// ErrAuthorizationDenied means a person refused the request.
var ErrAuthorizationDenied = errors.New("hub: the authorization request was refused")

// ErrAuthorizationExpired means the code expired before anyone approved it.
var ErrAuthorizationExpired = errors.New("hub: the authorization code expired before it was approved")

// StartDeviceAuthorization begins a flow.
func (c *Client) StartDeviceAuthorization(ctx context.Context) (*DeviceAuthorization, error) {
	var authz DeviceAuthorization
	if err := c.postJSON(ctx, "/api/v1/auth/device/code", map[string]any{}, &authz); err != nil {
		return nil, err
	}
	if authz.DeviceCode == "" || authz.UserCode == "" {
		return nil, fmt.Errorf("hub: %s returned an incomplete device authorization", c.BaseURL)
	}
	return &authz, nil
}

// tokenResponse is the success shape, matching what login and register return.
type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	OrgID     string `json:"org_id"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
}

// deviceError is the RFC 8628 §3.5 error shape.
type deviceError struct {
	Error string `json:"error"`
}

// PollForToken polls until the request is approved, refused, or expires.
//
// The interval rules are the part worth getting right, because they are what stops a client
// from being told to slow down and carrying on regardless:
//
//   - The wait starts at the interval Hub gave us, not at a constant of our own. Hub judges
//     slow_down against the interval it promised this client.
//   - slow_down increases the interval permanently for this flow, as the RFC requires. Treating
//     it as a one-off sleep means the next poll is early again and earns another slow_down.
//   - Deadline comes from expires_in, so the loop ends by itself. A client that polls forever
//     against an expired code is indistinguishable from a stuck one.
func (c *Client) PollForToken(ctx context.Context, authz *DeviceAuthorization) (*StoredToken, error) {
	interval := time.Duration(authz.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	lifetime := time.Duration(authz.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = 10 * time.Minute
	}
	deadline := time.Now().Add(lifetime)

	for {
		// Waited before the first poll on purpose: nobody can have approved a code in the
		// milliseconds since it was printed, so an immediate poll only costs a round trip and
		// starts the interval clock earlier than the person can act.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		if time.Now().After(deadline) {
			return nil, ErrAuthorizationExpired
		}

		var token tokenResponse
		err := c.postJSON(ctx, "/api/v1/auth/device/token",
			map[string]any{"device_code": authz.DeviceCode}, &token)
		if err == nil {
			if token.Token == "" {
				return nil, fmt.Errorf("hub: %s approved the request but returned no token", c.BaseURL)
			}
			expires := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			return &StoredToken{
				Token:     token.Token,
				HubURL:    c.BaseURL,
				OrgID:     token.OrgID,
				UserID:    token.UserID,
				ExpiresAt: expires,
			}, nil
		}

		var de *deviceFlowError
		if !errors.As(err, &de) {
			// A transport failure rather than a protocol answer. Kept going rather than
			// aborting: a flow that dies on one dropped connection makes the person start over
			// for no reason, and the deadline above still bounds the wait.
			continue
		}

		switch de.code {
		case "authorization_pending":
			// Nobody has decided yet.
		case "slow_down":
			// Permanent for this flow, per §3.5.
			interval += 5 * time.Second
		case "access_denied":
			return nil, ErrAuthorizationDenied
		case "expired_token":
			return nil, ErrAuthorizationExpired
		default:
			return nil, fmt.Errorf("hub: %s refused the request: %s", c.BaseURL, de.code)
		}
	}
}

// deviceFlowError carries an RFC 8628 error code as an error value.
type deviceFlowError struct {
	status int
	code   string
}

func (e *deviceFlowError) Error() string {
	return fmt.Sprintf("hub: device authorization: %s (HTTP %d)", e.code, e.status)
}

// postJSON sends a request and decodes the response, turning a device-flow error body into a
// deviceFlowError the caller can switch on.
func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("hub: encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("hub: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("hub: %s is unreachable: %w", c.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Bounded: this is an unauthenticated endpoint on a host the user named, and an
	// unbounded read would let a hostile or broken server exhaust memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("hub: reading the response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var de deviceError
		if json.Unmarshal(raw, &de) == nil && de.Error != "" {
			return &deviceFlowError{status: resp.StatusCode, code: de.Error}
		}
		return fmt.Errorf("hub: %s answered %s: %s", c.BaseURL, resp.Status, strings.TrimSpace(string(raw)))
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("hub: unreadable response from %s: %w", c.BaseURL, err)
		}
	}
	return nil
}

// readLimited reads a response body with a ceiling.
//
// Bounded because these endpoints are reached over a host the user named: an unbounded read lets
// a hostile or broken server exhaust memory in a CLI that was only asking a question.
func readLimited(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("hub: reading the response: %w", err)
	}
	return raw, nil
}
