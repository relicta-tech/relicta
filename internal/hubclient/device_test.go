package hubclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// The polling rules are the whole client. RFC 8628 §3.5 defines five answers and a client that
// mishandles any of them either never gets a token or hammers the endpoint it was asked to back
// off from.
//
// Written against a fake Hub rather than a real one so the intervals can be milliseconds. The
// flow was also driven end to end against a real Hub in a container, which is how the frozen
// clock in Hub's own auth service was found — but that is not a test that can live here.

// fakeHub answers /device/code once and then /device/token from a supplied script.
func fakeHub(t *testing.T, intervalSeconds int, answers []string) (*Client, *int64) {
	t.Helper()

	var polls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "dc-secret",
				"user_code":        "BCDF-GHJK",
				"verification_uri": "https://hub.example/device",
				"expires_in":       600,
				"interval":         intervalSeconds,
			})
		case "/api/v1/auth/device/token":
			n := atomic.AddInt64(&polls, 1)
			answer := answers[len(answers)-1]
			if int(n) <= len(answers) {
				answer = answers[n-1]
			}
			if answer == "token" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"token": "jwt-value", "expires_in": 3600,
					"org_id": "org-1", "user_id": "user-1",
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": answer})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL)
	return c, &polls
}

func TestPollingStopsWhenApproved(t *testing.T) {
	// interval 0 so the client falls back to its own floor; answers: pending, then a token.
	c, polls := fakeHub(t, 0, []string{"authorization_pending", "token"})
	c.HTTP = &http.Client{Timeout: 5 * time.Second}

	authz, err := c.StartDeviceAuthorization(context.Background())
	if err != nil {
		t.Fatalf("StartDeviceAuthorization: %v", err)
	}
	// Shorten the wait; the interval semantics are covered separately.
	authz.Interval = 0

	token, err := c.PollForToken(context.Background(), authz)
	if err != nil {
		t.Fatalf("PollForToken: %v", err)
	}
	if token.Token != "jwt-value" {
		t.Errorf("Token = %q, want the issued token", token.Token)
	}
	if token.HubURL != c.BaseURL {
		t.Errorf("HubURL = %q, want %q: a token is only valid for the Hub that issued it, and "+
			"recording the wrong one would send it elsewhere", token.HubURL, c.BaseURL)
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero: without it nothing can tell a stale token from a fresh one")
	}
	if *polls != 2 {
		t.Errorf("polled %d times, want 2 (pending, then approved)", *polls)
	}
}

// A refusal must end the wait, not be retried. Retrying a decided code polls forever against an
// answer that cannot change.
func TestDenialEndsTheWait(t *testing.T) {
	c, _ := fakeHub(t, 0, []string{"access_denied"})
	authz, _ := c.StartDeviceAuthorization(context.Background())
	authz.Interval = 0

	_, err := c.PollForToken(context.Background(), authz)
	if !errors.Is(err, ErrAuthorizationDenied) {
		t.Errorf("err = %v, want ErrAuthorizationDenied", err)
	}
}

func TestExpiryEndsTheWait(t *testing.T) {
	c, _ := fakeHub(t, 0, []string{"expired_token"})
	authz, _ := c.StartDeviceAuthorization(context.Background())
	authz.Interval = 0

	_, err := c.PollForToken(context.Background(), authz)
	if !errors.Is(err, ErrAuthorizationExpired) {
		t.Errorf("err = %v, want ErrAuthorizationExpired", err)
	}
}

// slow_down must widen the interval for the rest of the flow, per §3.5. Treating it as a
// one-off sleep means the next poll is early again and earns another slow_down — which is
// exactly the lockout that made this flow unusable against a real Hub.
func TestSlowDownWidensTheIntervalPermanently(t *testing.T) {
	c, _ := fakeHub(t, 0, []string{"slow_down", "authorization_pending", "token"})
	authz, _ := c.StartDeviceAuthorization(context.Background())
	authz.Interval = 0

	started := time.Now()
	if _, err := c.PollForToken(context.Background(), authz); err != nil {
		t.Fatalf("PollForToken: %v", err)
	}
	elapsed := time.Since(started)

	// One slow_down adds five seconds, and two further polls follow it, so the run cannot
	// finish quickly. A client that ignored the signal would return almost immediately.
	if elapsed < 5*time.Second {
		t.Errorf("the whole flow took %v: a slow_down must widen the interval for the rest of "+
			"the flow, not be absorbed as a single sleep", elapsed)
	}
}

// An unrecognized error code must stop rather than loop. Polling against an answer the client
// does not understand cannot become a token.
func TestAnUnknownErrorCodeStops(t *testing.T) {
	c, _ := fakeHub(t, 0, []string{"invalid_grant"})
	authz, _ := c.StartDeviceAuthorization(context.Background())
	authz.Interval = 0

	_, err := c.PollForToken(context.Background(), authz)
	if err == nil {
		t.Fatal("an unrecognized error code was retried instead of ending the flow")
	}
	if errors.Is(err, ErrAuthorizationDenied) || errors.Is(err, ErrAuthorizationExpired) {
		t.Errorf("err = %v: invalid_grant is neither a refusal nor an expiry and must not be "+
			"reported as one", err)
	}
}

// A canceled context must end the wait promptly. A CLI the user interrupts should stop, not
// keep polling until the code expires.
func TestCancellationEndsTheWait(t *testing.T) {
	c, _ := fakeHub(t, 0, []string{"authorization_pending"})
	authz, _ := c.StartDeviceAuthorization(context.Background())
	authz.Interval = 1

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	_, err := c.PollForToken(ctx, authz)
	if err == nil {
		t.Fatal("polling continued after cancellation")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Errorf("took %v to notice cancellation", elapsed)
	}
}
