package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileQueue_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "queue")

	q, err := NewFileQueue(queueDir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	for _, sub := range []string{"pending", "in_flight", "delivered", "dead_letter"} {
		path := filepath.Join(queueDir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}

	if q.Concurrency() != 1 {
		t.Errorf("default concurrency = %d, want 1", q.Concurrency())
	}
}

func TestNewFileQueue_WithConcurrency(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir, WithConcurrency(4))
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}
	if q.Concurrency() != 4 {
		t.Errorf("Concurrency() = %d, want 4", q.Concurrency())
	}
}

func TestEnqueue_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	d := &Delivery{
		ID:          "delivery-001",
		EndpointURL: "https://example.com/webhook",
		Payload:     json.RawMessage(`{"event":"test"}`),
		MaxAttempts: 5,
	}

	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Verify file exists on disk
	path := filepath.Join(dir, "pending", "delivery-001.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("delivery file not found: %v", err)
	}

	var persisted Delivery
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("failed to unmarshal persisted delivery: %v", err)
	}

	if persisted.ID != "delivery-001" {
		t.Errorf("persisted ID = %q, want %q", persisted.ID, "delivery-001")
	}
	if persisted.Status != StatusPending {
		t.Errorf("persisted Status = %q, want %q", persisted.Status, StatusPending)
	}
	if persisted.PayloadHash == "" {
		t.Error("persisted PayloadHash should not be empty")
	}
}

func TestEnqueue_RejectsEmptyID(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	d := &Delivery{
		EndpointURL: "https://example.com/webhook",
		Payload:     json.RawMessage(`{}`),
	}

	if err := q.Enqueue(d); err == nil {
		t.Fatal("Enqueue() should reject empty ID")
	}
}

func TestDequeue_ReturnsInFIFOOrder(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	for i := 0; i < 5; i++ {
		d := &Delivery{
			ID:          "delivery-" + string(rune('a'+i)),
			EndpointURL: endpoint,
			Payload:     json.RawMessage(`{"seq":` + string(rune('0'+i)) + `}`),
			MaxAttempts: 3,
		}
		if err := q.Enqueue(d); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		// Small delay to ensure distinct creation times
		time.Sleep(2 * time.Millisecond)
	}

	// Dequeue 3
	results, err := q.Dequeue(endpoint, 3)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Dequeue() returned %d items, want 3", len(results))
	}

	// Verify FIFO order
	for i, d := range results {
		expected := "delivery-" + string(rune('a'+i))
		if d.ID != expected {
			t.Errorf("results[%d].ID = %q, want %q", i, d.ID, expected)
		}
		if d.Status != StatusInFlight {
			t.Errorf("results[%d].Status = %q, want %q", i, d.Status, StatusInFlight)
		}
	}

	// Verify remaining pending count
	pending, err := q.ListPending(endpoint)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("remaining pending = %d, want 2", len(pending))
	}
}

func TestDequeue_FiltersByEndpoint(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoints := []struct {
		id  string
		url string
	}{
		{"d-endpoint-a", "https://a.com/hook"},
		{"d-endpoint-b", "https://b.com/hook"},
	}

	for _, ep := range endpoints {
		d := &Delivery{
			ID:          ep.id,
			EndpointURL: ep.url,
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 3,
		}
		if err := q.Enqueue(d); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}

	results, err := q.Dequeue("https://a.com/hook", 10)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Dequeue() returned %d items, want 1", len(results))
	}
}

func TestDequeue_RespectsNextAttemptAt(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"

	// Enqueue a delivery with future NextAttemptAt
	d := &Delivery{
		ID:            "delayed-delivery",
		EndpointURL:   endpoint,
		Payload:       json.RawMessage(`{}`),
		MaxAttempts:   3,
		NextAttemptAt: time.Now().UTC().Add(1 * time.Hour),
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Should not be dequeued because NextAttemptAt is in the future
	results, err := q.Dequeue(endpoint, 10)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Dequeue() returned %d items, want 0 (delivery not yet ready)", len(results))
	}
}

func TestComplete_MovesToDelivered(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	d := &Delivery{
		ID:          "complete-test",
		EndpointURL: endpoint,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 3,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	results, err := q.Dequeue(endpoint, 1)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}

	if err := q.Complete(results[0]); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Verify moved to delivered
	delivered, err := q.ListDelivered(endpoint)
	if err != nil {
		t.Fatalf("ListDelivered() error = %v", err)
	}
	if len(delivered) != 1 {
		t.Fatalf("ListDelivered() = %d, want 1", len(delivered))
	}
	if delivered[0].Status != StatusDelivered {
		t.Errorf("Status = %q, want %q", delivered[0].Status, StatusDelivered)
	}
}

func TestFail_ReturnsToPendingWithRetry(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	d := &Delivery{
		ID:          "fail-test",
		EndpointURL: endpoint,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 3,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	results, err := q.Dequeue(endpoint, 1)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}

	nextAttempt := time.Now().UTC().Add(10 * time.Second)
	if err := q.Fail(results[0], 500, "server error", nextAttempt); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	// Should be back in pending
	pending, err := q.ListPending(endpoint)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("ListPending() = %d, want 1", len(pending))
	}
	if pending[0].Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", pending[0].Attempts)
	}
	if pending[0].LastStatusCode != 500 {
		t.Errorf("LastStatusCode = %d, want 500", pending[0].LastStatusCode)
	}
	if pending[0].LastError != "server error" {
		t.Errorf("LastError = %q, want %q", pending[0].LastError, "server error")
	}
}

func TestFail_MovesToDeadLetterAfterMaxAttempts(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	d := &Delivery{
		ID:          "dead-letter-test",
		EndpointURL: endpoint,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 2,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Fail twice (max attempts = 2)
	for i := 0; i < 2; i++ {
		results, err := q.Dequeue(endpoint, 1)
		if err != nil {
			t.Fatalf("Dequeue() iteration %d error = %v", i, err)
		}
		if len(results) == 0 {
			t.Fatalf("Dequeue() iteration %d returned 0 items", i)
		}

		nextAttempt := time.Now().UTC()
		if err := q.Fail(results[0], 500, "server error", nextAttempt); err != nil {
			t.Fatalf("Fail() iteration %d error = %v", i, err)
		}
	}

	// Should be in dead letter
	deadLetter, err := q.ListDeadLetter(endpoint)
	if err != nil {
		t.Fatalf("ListDeadLetter() error = %v", err)
	}
	if len(deadLetter) != 1 {
		t.Fatalf("ListDeadLetter() = %d, want 1", len(deadLetter))
	}
	if deadLetter[0].Status != StatusDeadLetter {
		t.Errorf("Status = %q, want %q", deadLetter[0].Status, StatusDeadLetter)
	}
	if deadLetter[0].Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", deadLetter[0].Attempts)
	}
}

func TestGet_FindsDeliveryInAnyStatus(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	d := &Delivery{
		ID:          "get-test",
		EndpointURL: "https://example.com/webhook",
		Payload:     json.RawMessage(`{"test":true}`),
		MaxAttempts: 3,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	found, err := q.Get("get-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if found.ID != "get-test" {
		t.Errorf("Get().ID = %q, want %q", found.ID, "get-test")
	}
}

func TestGet_ReturnsErrorForMissing(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	_, err = q.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() should return error for nonexistent delivery")
	}
}

func TestRequeue_MovesDeadLetterToPending(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	d := &Delivery{
		ID:          "requeue-test",
		EndpointURL: endpoint,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 1,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Dequeue and fail to dead letter
	results, err := q.Dequeue(endpoint, 1)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if err := q.Fail(results[0], 500, "error", time.Now()); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	// Verify in dead letter
	dl, _ := q.ListDeadLetter(endpoint)
	if len(dl) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(dl))
	}

	// Requeue
	requeued, err := q.Requeue("requeue-test")
	if err != nil {
		t.Fatalf("Requeue() error = %v", err)
	}
	if requeued.Status != StatusPending {
		t.Errorf("requeued Status = %q, want %q", requeued.Status, StatusPending)
	}
	if requeued.Attempts != 0 {
		t.Errorf("requeued Attempts = %d, want 0 (reset)", requeued.Attempts)
	}

	// Verify back in pending
	pending, _ := q.ListPending(endpoint)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
}

func TestRecoverInFlight_MovesBackToPending(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	d := &Delivery{
		ID:          "recover-test",
		EndpointURL: endpoint,
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 3,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	// Dequeue to move to in_flight
	_, err = q.Dequeue(endpoint, 1)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}

	// Simulate restart: create a new queue instance
	q2, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	recovered, err := q2.RecoverInFlight()
	if err != nil {
		t.Fatalf("RecoverInFlight() error = %v", err)
	}
	if recovered != 1 {
		t.Errorf("RecoverInFlight() = %d, want 1", recovered)
	}

	// Verify back in pending
	pending, _ := q2.ListPending(endpoint)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after recovery, got %d", len(pending))
	}
}

func TestEndpoints_ReturnsUniqueEndpoints(t *testing.T) {
	dir := t.TempDir()
	q, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	// Enqueue multiple deliveries for same endpoint
	for i := 0; i < 3; i++ {
		d := &Delivery{
			ID:          "ep-test-" + string(rune('0'+i)),
			EndpointURL: "https://example.com/webhook",
			Payload:     json.RawMessage(`{}`),
			MaxAttempts: 3,
		}
		if err := q.Enqueue(d); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}

	// Enqueue one for a different endpoint
	d := &Delivery{
		ID:          "ep-test-other",
		EndpointURL: "https://other.com/webhook",
		Payload:     json.RawMessage(`{}`),
		MaxAttempts: 3,
	}
	if err := q.Enqueue(d); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	endpoints, err := q.Endpoints()
	if err != nil {
		t.Fatalf("Endpoints() error = %v", err)
	}
	if len(endpoints) != 2 {
		t.Errorf("Endpoints() = %d, want 2", len(endpoints))
	}
}

func TestHashPayload(t *testing.T) {
	d := &Delivery{
		Payload: json.RawMessage(`{"event":"test","data":"hello"}`),
	}
	d.HashPayload()

	if d.PayloadHash == "" {
		t.Fatal("HashPayload() should set PayloadHash")
	}

	// Same payload should produce same hash
	d2 := &Delivery{
		Payload: json.RawMessage(`{"event":"test","data":"hello"}`),
	}
	d2.HashPayload()

	if d.PayloadHash != d2.PayloadHash {
		t.Errorf("same payload should produce same hash: %q != %q", d.PayloadHash, d2.PayloadHash)
	}

	// Different payload should produce different hash
	d3 := &Delivery{
		Payload: json.RawMessage(`{"event":"different"}`),
	}
	d3.HashPayload()

	if d.PayloadHash == d3.PayloadHash {
		t.Error("different payloads should produce different hashes")
	}
}

func TestPersistence_SurvivesRestart(t *testing.T) {
	dir := t.TempDir()

	// Create queue and enqueue
	q1, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() error = %v", err)
	}

	endpoint := "https://example.com/webhook"
	for i := 0; i < 3; i++ {
		d := &Delivery{
			ID:          "persist-" + string(rune('0'+i)),
			EndpointURL: endpoint,
			Payload:     json.RawMessage(`{"seq":` + string(rune('0'+i)) + `}`),
			MaxAttempts: 5,
		}
		if err := q1.Enqueue(d); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
	}

	// "Restart" by creating a new queue instance on same directory
	q2, err := NewFileQueue(dir)
	if err != nil {
		t.Fatalf("NewFileQueue() after restart error = %v", err)
	}

	pending, err := q2.ListPending(endpoint)
	if err != nil {
		t.Fatalf("ListPending() after restart error = %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("after restart: pending = %d, want 3", len(pending))
	}
}
