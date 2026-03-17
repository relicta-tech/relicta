package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBroadcaster_BroadcastEvent(t *testing.T) {
	hub := NewHub([]string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Register a client
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	broadcaster := NewEventBroadcaster(hub)

	// Broadcast a custom event
	broadcaster.BroadcastEvent(EventApprovalRequested, map[string]any{
		"run_id":   "run-42",
		"approver": "alice",
	})

	select {
	case raw := <-client.send:
		var msg Message
		err := json.Unmarshal(raw, &msg)
		require.NoError(t, err)

		assert.Equal(t, EventApprovalRequested, msg.Type)
		payload, ok := msg.Payload.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "run-42", payload["run_id"])
		assert.Equal(t, "alice", payload["approver"])
		assert.NotEmpty(t, payload["timestamp"])

	case <-time.After(200 * time.Millisecond):
		t.Fatal("did not receive broadcast event")
	}
}

func TestEventBroadcaster_BroadcastEvent_NilPayload(t *testing.T) {
	hub := NewHub([]string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	broadcaster := NewEventBroadcaster(hub)
	broadcaster.BroadcastEvent(EventAnalyticsUpdated, nil)

	select {
	case raw := <-client.send:
		var msg Message
		err := json.Unmarshal(raw, &msg)
		require.NoError(t, err)

		assert.Equal(t, EventAnalyticsUpdated, msg.Type)
		payload, ok := msg.Payload.(map[string]any)
		require.True(t, ok)
		assert.NotEmpty(t, payload["timestamp"])

	case <-time.After(200 * time.Millisecond):
		t.Fatal("did not receive broadcast event")
	}
}

func TestEventConstants(t *testing.T) {
	// Verify event type constants are defined
	assert.Equal(t, "release.progress", EventReleaseProgress)
	assert.Equal(t, "approval.requested", EventApprovalRequested)
	assert.Equal(t, "approval.completed", EventApprovalCompleted)
	assert.Equal(t, "analytics.updated", EventAnalyticsUpdated)
}
