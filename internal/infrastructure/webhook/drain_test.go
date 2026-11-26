package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/config"
	"github.com/relicta-tech/relicta/v5/internal/domain/release"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
)

// Deliveries were sent with a bare `go` and nothing ever waited for them. relicta is one
// process per command and exits as soon as publish returns, so a delivery in flight was
// killed with the process — whether a webhook arrived depended on which won the race, and
// the loser left no error, no retry and no log. Close is what the container waits on
// during shutdown.
func TestCloseWaitsForDeliveriesInFlight(t *testing.T) {
	var delivered atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		// Hold the response open until the test starts shutdown.
		<-release
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	publisher := NewPublisher([]config.WebhookConfig{{
		Name:       "slow",
		URL:        server.URL,
		Timeout:    time.Second,
		RetryCount: 1,
	}}, nil)

	published := make(chan error, 1)
	go func() {
		published <- publisher.Publish(context.Background(), &domain.RunPublishedEvent{
			RunID: "run-1",
			At:    time.Now(),
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("webhook request did not start")
	}

	closed := make(chan error, 1)
	go func() {
		closed <- publisher.Close()
	}()

	// Close and Publish must still be waiting for the in-flight delivery.
	select {
	case <-closed:
		t.Fatal("Close returned while a delivery was still in flight: this is the bug — the " +
			"process exits here and the webhook is abandoned mid-request")
	case <-time.After(150 * time.Millisecond):
	}
	select {
	case err := <-published:
		t.Fatalf("Publish returned before the in-flight request completed: %v", err)
	default:
	}

	close(release)

	select {
	case err := <-published:
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Publish did not return after the delivery completed")
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the delivery completed")
	}

	if got := delivered.Load(); got != 1 {
		t.Errorf("delivered = %d, want 1", got)
	}
}

// The event vocabulary the configuration documents has to be the one events actually use.
// The config comment lists "release.published", "release.canceled" and offers "release.*"
// as the wildcard example, while events were named "run.*" — so a user who configured
// exactly what was documented received nothing, and there was nothing to log because
// shouldSendEvent simply matched no event.
func TestTheDocumentedEventNamesAreTheOnesEmitted(t *testing.T) {
	events := []release.DomainEvent{
		&domain.RunCreatedEvent{RunID: "r", At: time.Now()},
		&domain.RunPublishedEvent{RunID: "r", At: time.Now()},
		&domain.RunFailedEvent{RunID: "r", At: time.Now()},
		&domain.RunCanceledEvent{RunID: "r", At: time.Now()},
	}

	publisher := &Publisher{}
	wh := &config.WebhookConfig{Events: []string{"release.*"}}

	for _, event := range events {
		if !publisher.shouldSendEvent(wh, event.EventName()) {
			t.Errorf("a webhook configured with the documented %q filter does not match %q",
				"release.*", event.EventName())
		}
	}

	// And an explicitly named event, the other documented form.
	named := &config.WebhookConfig{Events: []string{"release.published"}}
	if !publisher.shouldSendEvent(named, (&domain.RunPublishedEvent{}).EventName()) {
		t.Error(`a webhook configured for "release.published" does not match the published event`)
	}
	if publisher.shouldSendEvent(named, (&domain.RunFailedEvent{}).EventName()) {
		t.Error(`a webhook configured for "release.published" also matched the failed event`)
	}
}
