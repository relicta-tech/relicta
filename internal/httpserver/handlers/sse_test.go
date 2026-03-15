package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSEHub(t *testing.T) {
	hub := NewSSEHub(0)
	require.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestSSEHub_BroadcastNoClients(t *testing.T) {
	hub := NewSSEHub(10)
	// Should not panic with no clients
	hub.Broadcast("test.event", map[string]string{"key": "value"})
}

func TestSSEHub_BroadcastToClients(t *testing.T) {
	hub := NewSSEHub(10)

	_, events, cleanup := hub.addClient()
	defer cleanup()

	assert.Equal(t, 1, hub.ClientCount())

	hub.Broadcast("test.event", map[string]string{"key": "value"})

	select {
	case evt := <-events:
		assert.Equal(t, "test.event", evt.Type)
		assert.Equal(t, uint64(1), evt.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("did not receive event in time")
	}
}

func TestSSEHub_MultipleClients(t *testing.T) {
	hub := NewSSEHub(10)

	_, events1, cleanup1 := hub.addClient()
	defer cleanup1()
	_, events2, cleanup2 := hub.addClient()
	defer cleanup2()

	assert.Equal(t, 2, hub.ClientCount())

	hub.Broadcast("multi.event", nil)

	select {
	case evt := <-events1:
		assert.Equal(t, "multi.event", evt.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client 1 did not receive event")
	}

	select {
	case evt := <-events2:
		assert.Equal(t, "multi.event", evt.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client 2 did not receive event")
	}
}

func TestSSEHub_ClientCleanup(t *testing.T) {
	hub := NewSSEHub(10)

	_, _, cleanup := hub.addClient()
	assert.Equal(t, 1, hub.ClientCount())

	cleanup()
	assert.Equal(t, 0, hub.ClientCount())
}

func TestSSEHub_ReplaySince(t *testing.T) {
	hub := NewSSEHub(10)

	hub.Broadcast("event.1", nil)
	hub.Broadcast("event.2", nil)
	hub.Broadcast("event.3", nil)

	// Replay from event ID 1 should return events 2 and 3
	events := hub.replaySince(1)
	assert.Len(t, events, 2)
	assert.Equal(t, "event.2", events[0].Type)
	assert.Equal(t, "event.3", events[1].Type)

	// Replay from 0 should return all
	events = hub.replaySince(0)
	assert.Len(t, events, 3)

	// Replay from latest should return none
	events = hub.replaySince(3)
	assert.Len(t, events, 0)
}

func TestSSEHub_ReplayBufferOverflow(t *testing.T) {
	hub := NewSSEHub(3)

	hub.Broadcast("event.1", nil)
	hub.Broadcast("event.2", nil)
	hub.Broadcast("event.3", nil)
	hub.Broadcast("event.4", nil) // Should evict event.1

	events := hub.replaySince(0)
	assert.Len(t, events, 3)
	assert.Equal(t, "event.2", events[0].Type)
}

func TestSSEHub_Close(t *testing.T) {
	hub := NewSSEHub(10)

	_, _, _ = hub.addClient()
	_, _, _ = hub.addClient()

	assert.Equal(t, 2, hub.ClientCount())

	hub.Close()

	assert.Equal(t, 0, hub.ClientCount())

	// Broadcast after close should not panic
	hub.Broadcast("after.close", nil)
}

func TestSSEStreamHandler_Headers(t *testing.T) {
	hub := NewSSEHub(10)
	handler := SSEStreamHandler(hub)

	// Use httptest.ResponseRecorder which implements http.Flusher
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	rec := httptest.NewRecorder()

	// Run handler in goroutine since it blocks
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(rec, req)
	}()

	// Give the handler time to set headers and write retry hint
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
	assert.Contains(t, rec.Body.String(), "retry: 5000")
}

func TestSSEHub_ReplaySince_WithLastEventID(t *testing.T) {
	// This tests the replay logic directly without HTTP streaming,
	// since streaming tests are inherently timing-sensitive.
	hub := NewSSEHub(10)

	hub.Broadcast("event.1", map[string]string{"v": "1"})
	hub.Broadcast("event.2", map[string]string{"v": "2"})
	hub.Broadcast("event.3", map[string]string{"v": "3"})

	// Simulate Last-Event-ID: 1 -- should replay events 2 and 3
	replayed := hub.replaySince(1)
	require.Len(t, replayed, 2)

	id1, err := strconv.ParseUint(strconv.FormatUint(replayed[0].ID, 10), 10, 64)
	require.NoError(t, err)
	assert.True(t, id1 > 1, "first replayed event ID should be > 1")

	assert.Equal(t, "event.2", replayed[0].Type)
	assert.Equal(t, "event.3", replayed[1].Type)
}

func TestSSEHub_IncrementingEventIDs(t *testing.T) {
	hub := NewSSEHub(10)

	_, events, cleanup := hub.addClient()
	defer cleanup()

	hub.Broadcast("a", nil)
	hub.Broadcast("b", nil)
	hub.Broadcast("c", nil)

	var ids []uint64
	for i := 0; i < 3; i++ {
		select {
		case evt := <-events:
			ids = append(ids, evt.ID)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("missing event")
		}
	}

	assert.Equal(t, uint64(1), ids[0])
	assert.Equal(t, uint64(2), ids[1])
	assert.Equal(t, uint64(3), ids[2])
}
