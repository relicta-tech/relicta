package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestPublisher_SendsToWebhook(t *testing.T) {
	var received *WebhookPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}

		var payload WebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
			http.Error(w, "unmarshal error", http.StatusBadRequest)
			return
		}

		received = &payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name: "test",
			URL:  server.URL,
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release-1")
	ver := version.MustParse("1.2.0")
	event := release.NewReleasePublishedEvent(releaseID, ver, "v1.2.0", "https://example.com/release")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if received == nil {
		t.Fatal("webhook did not receive payload")
	}

	if received.Event != "release.published" {
		t.Errorf("expected event 'release.published', got %q", received.Event)
	}

	if received.ReleaseID != "test-release-1" {
		t.Errorf("expected release_id 'test-release-1', got %q", received.ReleaseID)
	}

	if received.Data["version"] != "1.2.0" {
		t.Errorf("expected version '1.2.0', got %v", received.Data["version"])
	}

	if received.Data["tag_name"] != "v1.2.0" {
		t.Errorf("expected tag_name 'v1.2.0', got %v", received.Data["tag_name"])
	}
}

func TestPublisher_FiltersEvents(t *testing.T) {
	var receivedEvents []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		eventName := r.Header.Get("X-Relicta-Event")
		receivedEvents = append(receivedEvents, eventName)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name:   "filtered",
			URL:    server.URL,
			Events: []string{"release.published", "release.failed"},
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")

	events := []release.DomainEvent{
		release.NewReleaseInitializedEvent(releaseID, "main", "owner/repo"),
		release.NewReleaseApprovedEvent(releaseID, "admin"),
		release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", ""),
	}

	err := publisher.Publish(context.Background(), events...)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async sends
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Only release.published should be received (initialized and approved are filtered out)
	if len(receivedEvents) != 1 {
		t.Errorf("expected 1 event, got %d: %v", len(receivedEvents), receivedEvents)
	}

	if len(receivedEvents) > 0 && receivedEvents[0] != "release.published" {
		t.Errorf("expected 'release.published', got %q", receivedEvents[0])
	}
}

func TestPublisher_WildcardFilter(t *testing.T) {
	var receivedCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name:   "wildcard",
			URL:    server.URL,
			Events: []string{"release.*"},
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	events := []release.DomainEvent{
		release.NewReleaseInitializedEvent(releaseID, "main", "owner/repo"),
		release.NewReleaseApprovedEvent(releaseID, "admin"),
		release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", ""),
	}

	err := publisher.Publish(context.Background(), events...)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async sends
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedCount != 3 {
		t.Errorf("expected 3 events (wildcard match), got %d", receivedCount)
	}
}

func TestPublisher_SignsPayload(t *testing.T) {
	secret := "my-webhook-secret"
	var receivedSignature string
	var receivedBody []byte
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		receivedSignature = r.Header.Get("X-Relicta-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name:   "signed",
			URL:    server.URL,
			Secret: secret,
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	event := release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", "")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedSignature == "" {
		t.Fatal("expected X-Relicta-Signature header")
	}

	// Verify the signature is valid
	if !VerifySignature(receivedBody, receivedSignature, secret) {
		t.Error("signature verification failed")
	}
}

func TestPublisher_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedHeaders = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name: "custom-headers",
			URL:  server.URL,
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"Authorization":   "Bearer test-token",
			},
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	event := release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", "")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected X-Custom-Header 'custom-value', got %q", receivedHeaders.Get("X-Custom-Header"))
	}

	if receivedHeaders.Get("Authorization") != "Bearer test-token" {
		t.Errorf("expected Authorization 'Bearer test-token', got %q", receivedHeaders.Get("Authorization"))
	}
}

func TestPublisher_DisabledWebhook(t *testing.T) {
	var receivedCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disabled := false
	webhooks := []config.WebhookConfig{
		{
			Name:    "disabled",
			URL:     server.URL,
			Enabled: &disabled,
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	event := release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", "")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for potential send
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if receivedCount != 0 {
		t.Errorf("disabled webhook should not receive events, got %d", receivedCount)
	}
}

func TestPublisher_ForwardsToNextPublisher(t *testing.T) {
	nextPublisher := &mockEventPublisher{}

	webhooks := []config.WebhookConfig{}
	publisher := NewPublisher(webhooks, nextPublisher)

	releaseID := release.ReleaseID("test-release")
	event := release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", "")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(nextPublisher.events) != 1 {
		t.Fatalf("expected 1 event forwarded, got %d", len(nextPublisher.events))
	}

	if nextPublisher.events[0].EventName() != "release.published" {
		t.Errorf("expected 'release.published' event, got %q", nextPublisher.events[0].EventName())
	}
}

func TestPublisher_RetriesOnFailure(t *testing.T) {
	var attemptCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		count := attemptCount
		mu.Unlock()

		if count < 3 {
			http.Error(w, "temporary error", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name:       "retry-test",
			URL:        server.URL,
			RetryCount: 5,
			RetryDelay: 10 * time.Millisecond,
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	event := release.NewReleasePublishedEvent(releaseID, version.MustParse("1.0.0"), "v1.0.0", "")

	err := publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for retries
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if attemptCount != 3 {
		t.Errorf("expected 3 attempts (2 failures + 1 success), got %d", attemptCount)
	}
}

func TestPublisher_AllEventTypes(t *testing.T) {
	var receivedPayloads []*WebhookPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload WebhookPayload
		json.Unmarshal(body, &payload)

		mu.Lock()
		receivedPayloads = append(receivedPayloads, &payload)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhooks := []config.WebhookConfig{
		{
			Name: "all-events",
			URL:  server.URL,
		},
	}

	publisher := NewPublisher(webhooks, nil)

	releaseID := release.ReleaseID("test-release")
	events := []release.DomainEvent{
		release.NewReleaseInitializedEvent(releaseID, "main", "owner/repo"),
		release.NewReleasePlannedEvent(releaseID, version.MustParse("1.0.0"), version.MustParse("1.1.0"), "minor", 5),
		release.NewReleaseVersionedEvent(releaseID, version.MustParse("1.1.0"), "v1.1.0"),
		release.NewReleaseNotesGeneratedEvent(releaseID, true, 500),
		release.NewReleaseApprovedEvent(releaseID, "admin"),
		release.NewReleasePublishingStartedEvent(releaseID, []string{"github", "slack"}),
		release.NewReleasePublishedEvent(releaseID, version.MustParse("1.1.0"), "v1.1.0", "https://example.com"),
		release.NewReleaseFailedEvent(releaseID, "test failure", release.StatePublishing, true),
		release.NewReleaseCanceledEvent(releaseID, "user request", "admin"),
		release.NewPluginExecutedEvent(releaseID, "github", "PostPublish", true, "success", 2*time.Second),
		release.NewReleaseRetriedEvent(releaseID, release.StateFailed, release.StatePlanned),
	}

	err := publisher.Publish(context.Background(), events...)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for async sends
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedPayloads) != len(events) {
		t.Errorf("expected %d events, got %d", len(events), len(receivedPayloads))
	}

	// Verify each event type was properly serialized
	eventNames := make(map[string]bool)
	for _, p := range receivedPayloads {
		eventNames[p.Event] = true
	}

	expectedEvents := []string{
		"release.initialized",
		"release.planned",
		"release.versioned",
		"release.notes_generated",
		"release.approved",
		"release.publishing_started",
		"release.published",
		"release.failed",
		"release.canceled",
		"release.plugin_executed",
		"release.retried",
	}

	for _, expected := range expectedEvents {
		if !eventNames[expected] {
			t.Errorf("missing event %q in received payloads", expected)
		}
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"event":"release.published","release_id":"test"}`)
	secret := "test-secret"

	signature := signPayload(payload, secret)

	if !VerifySignature(payload, "sha256="+signature, secret) {
		t.Error("signature verification should pass with correct secret")
	}

	if VerifySignature(payload, "sha256="+signature, "wrong-secret") {
		t.Error("signature verification should fail with wrong secret")
	}

	if VerifySignature([]byte("different payload"), "sha256="+signature, secret) {
		t.Error("signature verification should fail with different payload")
	}
}

func TestWebhookConfig_Defaults(t *testing.T) {
	cfg := &config.WebhookConfig{
		Name: "test",
		URL:  "http://example.com",
	}

	if !cfg.IsWebhookEnabled() {
		t.Error("webhook should be enabled by default")
	}

	if getTimeout(cfg) != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", getTimeout(cfg))
	}

	if getRetryCount(cfg) != 3 {
		t.Errorf("expected default retry count 3, got %d", getRetryCount(cfg))
	}

	if getRetryDelay(cfg) != 1*time.Second {
		t.Errorf("expected default retry delay 1s, got %v", getRetryDelay(cfg))
	}
}

type mockEventPublisher struct {
	events []release.DomainEvent
}

func (m *mockEventPublisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	m.events = append(m.events, events...)
	return nil
}
