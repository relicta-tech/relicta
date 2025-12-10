// Package persistence provides infrastructure implementations for data persistence.
package persistence

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestInMemoryEventPublisher_Publish(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")

	err := publisher.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	events := publisher.GetEvents()
	if len(events) != 1 {
		t.Errorf("GetEvents() length = %d, want 1", len(events))
	}
}

func TestInMemoryEventPublisher_PublishMultiple(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	event1 := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	event2 := release.NewReleasePlannedEvent("test-1", version.MustParse("1.0.0"), version.MustParse("1.1.0"), changes.ReleaseTypeMinor.String(), 5)

	err := publisher.Publish(ctx, event1, event2)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	events := publisher.GetEvents()
	if len(events) != 2 {
		t.Errorf("GetEvents() length = %d, want 2", len(events))
	}
}

func TestInMemoryEventPublisher_Subscribe(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	var receivedEvents []release.DomainEvent
	var mu sync.Mutex

	publisher.Subscribe(func(event release.DomainEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	err := publisher.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedEvents) != 1 {
		t.Errorf("Handler received %d events, want 1", len(receivedEvents))
	}
}

func TestInMemoryEventPublisher_MultipleSubscribers(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	var count1, count2 int32

	publisher.Subscribe(func(_ release.DomainEvent) {
		atomic.AddInt32(&count1, 1)
	})
	publisher.Subscribe(func(_ release.DomainEvent) {
		atomic.AddInt32(&count2, 1)
	})

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	err := publisher.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if atomic.LoadInt32(&count1) != 1 {
		t.Errorf("Handler 1 received %d events, want 1", count1)
	}
	if atomic.LoadInt32(&count2) != 1 {
		t.Errorf("Handler 2 received %d events, want 1", count2)
	}
}

func TestInMemoryEventPublisher_ClearEvents(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	_ = publisher.Publish(ctx, event)

	if len(publisher.GetEvents()) != 1 {
		t.Fatal("Expected 1 event before clear")
	}

	publisher.ClearEvents()

	if len(publisher.GetEvents()) != 0 {
		t.Errorf("GetEvents() length = %d after clear, want 0", len(publisher.GetEvents()))
	}
}

func TestInMemoryEventPublisher_GetEventsByType(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	event1 := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	event2 := release.NewReleaseApprovedEvent("test-1", "user")
	event3 := release.NewReleaseInitializedEvent("test-2", "develop", "/repo2")

	_ = publisher.Publish(ctx, event1, event2, event3)

	initEvents := publisher.GetEventsByType("release.initialized")
	if len(initEvents) != 2 {
		t.Errorf("GetEventsByType() length = %d, want 2", len(initEvents))
	}

	approveEvents := publisher.GetEventsByType("release.approved")
	if len(approveEvents) != 1 {
		t.Errorf("GetEventsByType() length = %d, want 1", len(approveEvents))
	}
}

func TestInMemoryEventPublisher_GetEventsByAggregateID(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	event1 := release.NewReleaseInitializedEvent("release-1", "main", "/repo")
	event2 := release.NewReleaseApprovedEvent("release-1", "user")
	event3 := release.NewReleaseInitializedEvent("release-2", "main", "/repo")

	_ = publisher.Publish(ctx, event1, event2, event3)

	release1Events := publisher.GetEventsByAggregateID("release-1")
	if len(release1Events) != 2 {
		t.Errorf("GetEventsByAggregateID() length = %d, want 2", len(release1Events))
	}

	release2Events := publisher.GetEventsByAggregateID("release-2")
	if len(release2Events) != 1 {
		t.Errorf("GetEventsByAggregateID() length = %d, want 1", len(release2Events))
	}
}

func TestInMemoryEventPublisher_ConcurrentPublish(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	const numGoroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := release.NewReleaseInitializedEvent(release.ReleaseID("test-"+string(rune(idx))), "main", "/repo")
			_ = publisher.Publish(ctx, event)
		}(i)
	}

	wg.Wait()

	events := publisher.GetEvents()
	if len(events) != numGoroutines {
		t.Errorf("GetEvents() length = %d, want %d", len(events), numGoroutines)
	}
}

func TestInMemoryEventPublisher_HandlerDoesNotBlockOtherOperations(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	// Subscribe a slow handler
	slowHandlerStarted := make(chan struct{})
	slowHandlerDone := make(chan struct{})

	publisher.Subscribe(func(_ release.DomainEvent) {
		close(slowHandlerStarted)
		time.Sleep(100 * time.Millisecond)
		close(slowHandlerDone)
	})

	// Publish an event (will trigger slow handler)
	go func() {
		event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
		_ = publisher.Publish(ctx, event)
	}()

	// Wait for slow handler to start
	<-slowHandlerStarted

	// Try to read events while handler is running - this should NOT block
	// because the lock is released before calling handlers
	done := make(chan struct{})
	go func() {
		_ = publisher.GetEvents()
		close(done)
	}()

	select {
	case <-done:
		// Success - GetEvents didn't block
	case <-time.After(50 * time.Millisecond):
		t.Error("GetEvents() blocked while handler was running - lock contention detected")
	}

	// Wait for handler to complete
	<-slowHandlerDone
}

func TestInMemoryEventPublisher_HandlerCanCallPublisher(t *testing.T) {
	publisher := NewInMemoryEventPublisher()
	ctx := context.Background()

	// Subscribe a handler that calls back into the publisher
	publisher.Subscribe(func(_ release.DomainEvent) {
		// This should not deadlock
		_ = publisher.GetEvents()
	})

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")

	done := make(chan struct{})
	go func() {
		_ = publisher.Publish(ctx, event)
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(1 * time.Second):
		t.Error("Publish() deadlocked when handler called back into publisher")
	}
}

func TestNoOpEventPublisher_Publish(t *testing.T) {
	publisher := NewNoOpEventPublisher()
	ctx := context.Background()

	event := release.NewReleaseInitializedEvent("test-1", "main", "/repo")
	err := publisher.Publish(ctx, event)

	if err != nil {
		t.Errorf("Publish() error = %v, want nil", err)
	}
}
