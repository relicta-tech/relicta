package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/release"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v5/internal/infrastructure/webhook/queue"
)

type eventPublisherFunc func(context.Context, ...release.DomainEvent) error

func (f eventPublisherFunc) Publish(ctx context.Context, events ...release.DomainEvent) error {
	return f(ctx, events...)
}

func TestPersistentPublisherQueuesForwardsAndProcessesDelivery(t *testing.T) {
	var gotPayload WebhookPayload
	var gotSignature string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode webhook payload: %v", err)
		}
		gotSignature = r.Header.Get("X-Relicta-Signature")
		if r.Header.Get("X-Relicta-Event") != "release.published" {
			t.Errorf("event header = %q", r.Header.Get("X-Relicta-Event"))
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("custom header = %q", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	q, err := queue.NewFileQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileQueue: %v", err)
	}
	forwarded := 0
	publisher := NewPersistentPublisher([]config.WebhookConfig{{
		Name: "release-hook", URL: server.URL, Secret: "test-secret",
		Headers: map[string]string{"X-Custom": "value"}, Events: []string{"release.published"},
		RetryCount: 1, Timeout: time.Second,
	}}, eventPublisherFunc(func(_ context.Context, events ...release.DomainEvent) error {
		forwarded += len(events)
		return nil
	}), q)
	event := &domain.RunPublishedEvent{RunID: "run-42", At: time.Now().UTC()}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if forwarded != 1 {
		t.Fatalf("forwarded events = %d, want 1", forwarded)
	}
	pending, err := q.ListPending(server.URL)
	if err != nil || len(pending) != 1 {
		t.Fatalf("ListPending = %d deliveries, %v; want one", len(pending), err)
	}
	if pending[0].MaxAttempts != 2 || pending[0].Secret != "test-secret" || pending[0].Headers["X-Custom"] != "value" {
		t.Fatalf("queued settings = %#v", pending[0])
	}

	processed, err := publisher.processQueue(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("processQueue = %d, %v; want one processed delivery", processed, err)
	}
	if gotPayload.Event != "release.published" || gotPayload.ReleaseID != "run-42" {
		t.Fatalf("delivered payload = %#v", gotPayload)
	}
	if gotSignature == "" {
		t.Fatal("signed webhook did not include X-Relicta-Signature")
	}
	delivered, err := q.ListDelivered(server.URL)
	if err != nil || len(delivered) != 1 {
		t.Fatalf("ListDelivered = %d deliveries, %v; want one", len(delivered), err)
	}
}

func TestPersistentPublisherDeadLettersFailedDelivery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	q, err := queue.NewFileQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileQueue: %v", err)
	}
	publisher := NewPersistentPublisher([]config.WebhookConfig{{
		Name: "failing-hook", URL: server.URL, RetryCount: 1, RetryDelay: time.Millisecond,
	}}, nil, q)
	if err := publisher.Publish(context.Background(), &domain.RunPublishedEvent{RunID: "run-fail", At: time.Now()}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	processed, err := publisher.processQueue(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("processQueue = %d, %v; want one processed delivery", processed, err)
	}
	time.Sleep(5 * time.Millisecond)
	processed, err = publisher.processQueue(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("retry processQueue = %d, %v; want one processed delivery", processed, err)
	}
	dead, err := q.ListDeadLetter(server.URL)
	if err != nil || len(dead) != 1 {
		t.Fatalf("ListDeadLetter = %d deliveries, %v; want one", len(dead), err)
	}
	if dead[0].Attempts != 2 || dead[0].LastStatusCode != http.StatusServiceUnavailable || dead[0].LastError == "" {
		t.Fatalf("dead-letter result = %#v", dead[0])
	}
}

func TestPersistentPublisherStartIsIdempotentAndCloseReleasesWorker(t *testing.T) {
	q, err := queue.NewFileQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileQueue: %v", err)
	}
	publisher := NewPersistentPublisher(nil, nil, q)
	ctx, cancel := context.WithCancel(context.Background())
	publisher.Start(ctx)
	if !q.OwnsWorker() {
		t.Fatal("Start did not acquire the queue worker lock")
	}
	firstDone := publisher.workerDone
	publisher.Start(ctx)
	if publisher.workerDone != firstDone {
		t.Fatal("second Start replaced the active worker")
	}
	cancel()
	if err := publisher.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if q.OwnsWorker() {
		t.Fatal("Close did not release the queue worker lock")
	}
}
