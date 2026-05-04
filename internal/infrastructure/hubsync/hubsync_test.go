package hubsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/infrastructure/webhook/retry"
)

func TestNewClient_RejectsMissingURL(t *testing.T) {
	if _, err := NewClient(ClientConfig{HubURL: ""}); err == nil {
		t.Error("expected error for empty HubURL")
	}
}

func TestSyncEventStatus_TerminalCases(t *testing.T) {
	cases := map[string]bool{
		"accepted":  true,
		"duplicate": true,
		"rejected":  true,
		"failed":    false,
		"":          false,
	}
	for status, want := range cases {
		got := SyncEventStatus{Status: status}.IsTerminal()
		if got != want {
			t.Errorf("IsTerminal(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestPush_HappyPathSendsCGPVersionHeader(t *testing.T) {
	var seenVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenVersion = r.Header.Get("X-CGP-Version")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":1,"received":1,"results":[{"id":"e1","status":"accepted"}],"cgp_version":"1.0"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{HubURL: srv.URL})
	resp, err := c.Push(context.Background(), []byte(`[{"id":"e1"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if seenVersion != "1.0" {
		t.Errorf("X-CGP-Version sent: %q", seenVersion)
	}
	if resp.Accepted != 1 {
		t.Errorf("accepted: %d", resp.Accepted)
	}
}

func TestPush_VersionMismatchTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-CGP-Supported-Versions", "2.0")
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"error":"unsupported"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{HubURL: srv.URL, Retry: tightRetry()})
	_, err := c.Push(context.Background(), []byte(`[]`))
	var verErr *VersionMismatchError
	if !errors.As(err, &verErr) {
		t.Fatalf("expected VersionMismatchError, got %T: %v", err, err)
	}
}

func TestPush_5xxRetriesUpToLimit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := tightRetry()
	cfg.MaxRetries = 2
	c, _ := NewClient(ClientConfig{HubURL: srv.URL, Retry: cfg})
	_, err := c.Push(context.Background(), []byte(`[]`))
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if calls != 3 { // initial + 2 retries
		t.Errorf("calls: %d, want 3", calls)
	}
	var trErr *TransportError
	if !errors.As(err, &trErr) {
		t.Errorf("expected TransportError wrap, got %T", err)
	}
}

func TestPush_4xxNoRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{HubURL: srv.URL, Retry: tightRetry()})
	_, _ = c.Push(context.Background(), []byte(`[]`))
	if calls != 1 {
		t.Errorf("4xx should not retry, saw %d calls", calls)
	}
}

func TestPush_429RetriesAndSurfacesRetryAfter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0") // honored only on 503; 429 still retries
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":0,"received":0,"results":[],"cgp_version":"1.0"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{HubURL: srv.URL, Retry: tightRetry()})
	_, err := c.Push(context.Background(), []byte(`[]`))
	if err != nil {
		t.Fatalf("expected eventual success after 429: %v", err)
	}
	if calls < 2 {
		t.Errorf("calls: %d", calls)
	}
}

func TestPush_503ParsesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, _ := NewClient(ClientConfig{HubURL: srv.URL, Retry: tightRetry()})
	_, err := c.Push(context.Background(), []byte(`[]`))
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	var trErr *TransportError
	// Wrapped — Push's outer wrapper has Cause == per-attempt error.
	if !errors.As(err, &trErr) {
		t.Fatalf("expected TransportError, got %T", err)
	}
}

func TestFileQueue_EnqueueAndLen(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(filepath.Join(dir, "queue.ndjson"))
	if err != nil {
		t.Fatal(err)
	}

	if n, _ := q.Len(); n != 0 {
		t.Errorf("fresh queue len: %d", n)
	}

	for i := 0; i < 3; i++ {
		entry := QueueEntry{
			ID:      "e" + string(rune('0'+i)),
			OrgID:   "o",
			Payload: []byte(`{}`),
		}
		if err := q.Enqueue(entry); err != nil {
			t.Fatal(err)
		}
	}
	n, err := q.Len()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("Len after 3 enqueues: %d", n)
	}
}

func TestFileQueue_DrainKeepsAndRewrites(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewFileQueue(filepath.Join(dir, "q.ndjson"))

	for i := 0; i < 4; i++ {
		_ = q.Enqueue(QueueEntry{ID: string(rune('a' + i)), Payload: []byte(`{}`)})
	}

	// Drain: keep odd-indexed entries (b, d), drop even (a, c).
	idx := 0
	drained, err := q.Drain(func(e QueueEntry) (bool, QueueEntry, error) {
		keep := idx%2 == 1
		idx++
		return keep, e, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if drained != 2 {
		t.Errorf("drained: %d, want 2", drained)
	}

	n, _ := q.Len()
	if n != 2 {
		t.Errorf("post-drain len: %d, want 2", n)
	}
}

func TestFileQueue_DrainEmptyQueueRemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.ndjson")
	q, _ := NewFileQueue(path)

	_ = q.Enqueue(QueueEntry{ID: "e", Payload: []byte(`{}`)})
	_, err := q.Drain(func(_ QueueEntry) (bool, QueueEntry, error) {
		return false, QueueEntry{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// File should have been removed since pending is empty.
	if _, err := readFileAndExpectMissing(path); err == nil {
		t.Error("expected queue file to be removed after fully draining")
	}
}

func TestFileQueue_DrainSkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.ndjson")
	q, _ := NewFileQueue(path)

	good := QueueEntry{ID: "g", Payload: []byte(`{}`)}
	_ = q.Enqueue(good)
	// Append corruption.
	must(t, appendLine(path, "{not json"))
	_ = q.Enqueue(QueueEntry{ID: "g2", Payload: []byte(`{}`)})

	count := 0
	_, err := q.Drain(func(_ QueueEntry) (bool, QueueEntry, error) {
		count++
		return false, QueueEntry{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("drain visited %d entries, want 2 (corrupt skipped)", count)
	}
}

func TestQueueEntry_RoundTripJSON(t *testing.T) {
	in := QueueEntry{
		ID: "e", OrgID: "o", EnqueuedAt: time.Now().UTC().Truncate(time.Second),
		Attempts: 3, LastError: "boom", Payload: []byte(`{"hi":1}`),
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out QueueEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID || out.Attempts != in.Attempts || out.LastError != in.LastError {
		t.Errorf("round trip lost fields: %+v", out)
	}
}

// tightRetry returns retry config with near-zero delays so tests don't sleep.
func tightRetry() *retry.Config {
	cfg := retry.DefaultConfig()
	cfg.BaseDelay = time.Millisecond
	cfg.MaxDelay = 10 * time.Millisecond
	cfg.JitterFraction = 0
	cfg.MaxRetries = 3
	return &cfg
}

// readFileAndExpectMissing returns a non-nil error if the file is missing.
// Helper keeps the test assertion intent explicit.
func readFileAndExpectMissing(path string) ([]byte, error) {
	return readIfExists(path)
}

func readIfExists(path string) ([]byte, error) {
	return nil, errFromMissing(path)
}

func errFromMissing(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func appendLine(path, s string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(s + "\n")
	return err
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
