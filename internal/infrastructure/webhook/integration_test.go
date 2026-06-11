package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/infrastructure/webhook/queue"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/webhook/retry"
)

// TestIntegration_DeliveryWithRetries tests the full delivery pipeline:
// queue -> send -> fail -> retry with backoff -> succeed.
func TestIntegration_DeliveryWithRetries(t *testing.T) {
	var attemptCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attemptCount.Add(1)
		if count < 3 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	q, err := queue.NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	deliveryLog, err := queue.NewDeliveryLog(dir)
	if err != nil {
		t.Fatalf("NewDeliveryLog() error = %v", err)
	}

	retryCfg := retry.Config{
		MaxRetries:     5,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		JitterFraction: 0,
	}

	// Enqueue a delivery
	d := &queue.Delivery{
		ID:          "integration-test-001",
		EndpointURL: server.URL,
		Payload:     json.RawMessage(`{"event":"test","data":"hello"}`),
		MaxAttempts: retryCfg.MaxRetries,
		Event:       "test.event",
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Process the delivery with retries
	ctx := context.Background()
	client := &http.Client{Timeout: 5 * time.Second}

	for attempt := 0; attempt < retryCfg.MaxRetries; attempt++ {
		items, err := q.Dequeue(server.URL, 1)
		if err != nil {
			t.Fatalf("Dequeue() attempt %d error = %v", attempt, err)
		}
		if len(items) == 0 {
			// Wait for NextAttemptAt
			time.Sleep(20 * time.Millisecond)
			items, err = q.Dequeue(server.URL, 1)
			if err != nil {
				t.Fatalf("Dequeue() retry attempt %d error = %v", attempt, err)
			}
			if len(items) == 0 {
				continue
			}
		}

		item := items[0]

		// Send the request
		start := time.Now()
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, item.EndpointURL, nil)
		resp, err := client.Do(req)

		responseTime := time.Since(start).Milliseconds()

		logEntry := queue.DeliveryLogEntry{
			DeliveryID:     item.ID,
			Timestamp:      time.Now().UTC(),
			EndpointURL:    item.EndpointURL,
			ResponseTimeMS: responseTime,
			PayloadHash:    item.PayloadHash,
			Attempt:        item.Attempts + 1,
			Event:          item.Event,
		}

		if err != nil {
			logEntry.Error = err.Error()
			_ = deliveryLog.Record(logEntry)
			nextAttempt := retryCfg.NextAttemptTime(item.Attempts, nil)
			_ = q.Fail(item, 0, err.Error(), nextAttempt)
			continue
		}
		resp.Body.Close()

		logEntry.StatusCode = resp.StatusCode
		logEntry.Success = resp.StatusCode < 400

		_ = deliveryLog.Record(logEntry)

		if resp.StatusCode >= 400 {
			nextAttempt := retryCfg.NextAttemptTime(item.Attempts, resp)
			_ = q.Fail(item, resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode), nextAttempt)
		} else {
			_ = q.Complete(item)
			break
		}
	}

	// Verify: delivery should be completed
	delivered, err := q.ListDelivered(server.URL)
	if err != nil {
		t.Fatalf("ListDelivered() error = %v", err)
	}
	if len(delivered) != 1 {
		t.Errorf("expected 1 delivered, got %d", len(delivered))
	}

	// Verify: log should have 3 entries (2 failures + 1 success)
	logEntries, _ := deliveryLog.List(server.URL)
	if len(logEntries) != 3 {
		t.Errorf("expected 3 log entries, got %d", len(logEntries))
	}

	// Verify: total attempts on server
	if attemptCount.Load() != 3 {
		t.Errorf("expected 3 server attempts, got %d", attemptCount.Load())
	}
}

// TestIntegration_DeadLetterAfterMaxRetries tests that deliveries go to
// the dead-letter queue after exhausting all retry attempts.
func TestIntegration_DeadLetterAfterMaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "always fails", http.StatusInternalServerError)
	}))
	defer server.Close()

	dir := t.TempDir()
	q, err := queue.NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	maxRetries := 3
	d := &queue.Delivery{
		ID:          "dead-letter-integration",
		EndpointURL: server.URL,
		Payload:     json.RawMessage(`{"event":"test"}`),
		MaxAttempts: maxRetries,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	for attempt := 0; attempt < maxRetries; attempt++ {
		items, err := q.Dequeue(server.URL, 1)
		if err != nil {
			t.Fatalf("Dequeue() attempt %d error = %v", attempt, err)
		}
		if len(items) == 0 {
			t.Fatalf("Dequeue() attempt %d returned 0 items", attempt)
		}

		item := items[0]
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, item.EndpointURL, nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}

		nextAttempt := time.Now().UTC() // immediate retry for test
		_ = q.Fail(item, 500, "server error", nextAttempt)
	}

	// Verify dead letter
	dl, err := q.ListDeadLetter(server.URL)
	if err != nil {
		t.Fatalf("ListDeadLetter() error = %v", err)
	}
	if len(dl) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl))
	}
	if dl[0].Attempts != maxRetries {
		t.Errorf("dead letter attempts = %d, want %d", dl[0].Attempts, maxRetries)
	}

	// Verify no pending
	pending, _ := q.ListPending(server.URL)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending, got %d", len(pending))
	}
}

// TestIntegration_RetryAfterHeader tests that the Retry-After header
// from the server is respected.
func TestIntegration_RetryAfterHeader(t *testing.T) {
	var mu sync.Mutex
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		count := attempts
		mu.Unlock()

		if count == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryCfg := retry.Config{
		MaxRetries:     5,
		BaseDelay:      10 * time.Millisecond, // Very short base
		MaxDelay:       5 * time.Minute,
		JitterFraction: 0,
	}

	// First attempt with Retry-After response
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "1")

	retryAfter := retry.ParseRetryAfter(resp)
	if retryAfter != 1*time.Second {
		t.Errorf("ParseRetryAfter = %v, want 1s", retryAfter)
	}

	// NextAttemptTime should use the larger of backoff and Retry-After
	nextTime := retryCfg.NextAttemptTime(0, resp)
	expectedMinimum := time.Now().UTC().Add(900 * time.Millisecond) // ~1s minus small tolerance

	if nextTime.Before(expectedMinimum) {
		t.Errorf("NextAttemptTime should respect Retry-After (1s), but got %v from now",
			time.Until(nextTime))
	}

	server.Close()
}

// TestIntegration_SlowResponse tests handling of slow webhook endpoints.
func TestIntegration_SlowResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	start := time.Now()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if elapsed < 200*time.Millisecond {
		t.Errorf("request completed too fast: %v (expected >= 200ms)", elapsed)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestIntegration_RedirectHandling tests that webhook clients follow redirects.
func TestIntegration_RedirectHandling(t *testing.T) {
	var finalHit atomic.Bool

	// Final endpoint
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		finalHit.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer finalServer.Close()

	// Redirect endpoint
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusTemporaryRedirect)
	}))
	defer redirectServer.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, redirectServer.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if !finalHit.Load() {
		t.Error("redirect was not followed to final endpoint")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}
}

// TestIntegration_QueueRecoveryAfterRestart tests that deliveries
// persist across queue restarts.
func TestIntegration_QueueRecoveryAfterRestart(t *testing.T) {
	dir := t.TempDir()
	endpoint := "https://example.com/webhook"

	// Phase 1: Enqueue and start processing
	q1, err := queue.NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() phase 1 error = %v", err)
	}

	for i := 0; i < 3; i++ {
		d := &queue.Delivery{
			ID:          fmt.Sprintf("restart-test-%d", i),
			EndpointURL: endpoint,
			Payload:     json.RawMessage(fmt.Sprintf(`{"seq":%d}`, i)),
			MaxAttempts: 5,
		}
		if err := q1.Enqueue(d); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}

	// Dequeue one (simulating it was being processed when crash happened)
	_, err = q1.Dequeue(endpoint, 1)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}

	// Phase 2: "Restart" with new queue instance
	q2, err := queue.NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() phase 2 error = %v", err)
	}

	// Recover in-flight deliveries
	recovered, err := q2.RecoverInFlight()
	if err != nil {
		t.Fatalf("RecoverInFlight() error = %v", err)
	}
	if recovered != 1 {
		t.Errorf("recovered = %d, want 1", recovered)
	}

	// All 3 should be pending again
	pending, err := q2.ListPending(endpoint)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("pending after recovery = %d, want 3", len(pending))
	}
}

// TestIntegration_RedeliveryFromDeadLetter tests manual re-delivery
// of dead-lettered items.
func TestIntegration_RedeliveryFromDeadLetter(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count <= 2 {
			http.Error(w, "failing", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	q, err := queue.NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	d := &queue.Delivery{
		ID:          "redeliver-test",
		EndpointURL: server.URL,
		Payload:     json.RawMessage(`{"event":"test"}`),
		MaxAttempts: 1, // Will dead-letter after first failure
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Dequeue and fail to dead letter
	items, _ := q.Dequeue(server.URL, 1)
	_ = q.Fail(items[0], 500, "error", time.Now())

	// Verify in dead letter
	dl, _ := q.ListDeadLetter(server.URL)
	if len(dl) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl))
	}

	// Requeue from dead letter
	requeued, err := q.Requeue("redeliver-test")
	if err != nil {
		t.Fatalf("Requeue() error = %v", err)
	}
	if requeued.Attempts != 0 {
		t.Errorf("requeued attempts = %d, want 0 (reset)", requeued.Attempts)
	}

	// Verify back in pending
	pending, _ := q.ListPending(server.URL)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after requeue, got %d", len(pending))
	}

	// Dead letter should be empty
	dl, _ = q.ListDeadLetter(server.URL)
	if len(dl) != 0 {
		t.Errorf("expected 0 dead letters after requeue, got %d", len(dl))
	}
}

// TestIntegration_TimeoutHandling tests that webhook delivery respects
// HTTP client timeouts.
func TestIntegration_TimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
