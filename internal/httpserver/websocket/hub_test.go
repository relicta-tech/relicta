package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHub tests hub creation with various configurations.
func TestNewHub(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
	}{
		{
			name:           "with nil origins",
			allowedOrigins: nil,
		},
		{
			name:           "with empty origins",
			allowedOrigins: []string{},
		},
		{
			name:           "with specific origins",
			allowedOrigins: []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:           "with wildcard",
			allowedOrigins: []string{"*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub(tt.allowedOrigins)
			require.NotNil(t, hub)
			assert.NotNil(t, hub.clients)
			assert.NotNil(t, hub.broadcast)
			assert.NotNil(t, hub.register)
			assert.NotNil(t, hub.unregister)
			assert.Equal(t, tt.allowedOrigins, hub.allowedOrigins)
		})
	}
}

// TestHub_checkOrigin tests origin validation.
func TestHub_checkOrigin(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		origin         string
		expected       bool
	}{
		{
			name:           "no origin header - same origin",
			allowedOrigins: []string{"http://localhost:3000"},
			origin:         "",
			expected:       true,
		},
		{
			name:           "no configured origins - reject cross-origin",
			allowedOrigins: nil,
			origin:         "http://evil.com",
			expected:       false,
		},
		{
			name:           "empty configured origins - reject cross-origin",
			allowedOrigins: []string{},
			origin:         "http://evil.com",
			expected:       false,
		},
		{
			name:           "wildcard - allow all",
			allowedOrigins: []string{"*"},
			origin:         "http://any-site.com",
			expected:       true,
		},
		{
			name:           "matching origin",
			allowedOrigins: []string{"http://localhost:3000"},
			origin:         "http://localhost:3000",
			expected:       true,
		},
		{
			name:           "matching origin case-insensitive",
			allowedOrigins: []string{"http://LOCALHOST:3000"},
			origin:         "http://localhost:3000",
			expected:       true,
		},
		{
			name:           "matching https origin",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "non-matching origin",
			allowedOrigins: []string{"http://localhost:3000"},
			origin:         "http://evil.com",
			expected:       false,
		},
		{
			name:           "different port",
			allowedOrigins: []string{"http://localhost:3000"},
			origin:         "http://localhost:4000",
			expected:       false,
		},
		{
			name:           "different scheme",
			allowedOrigins: []string{"https://example.com"},
			origin:         "http://example.com",
			expected:       false,
		},
		{
			name:           "multiple allowed - match second",
			allowedOrigins: []string{"http://localhost:3000", "https://example.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "invalid origin URL",
			allowedOrigins: []string{"http://localhost:3000"},
			origin:         "://invalid",
			expected:       false,
		},
		{
			name:           "invalid allowed origin URL skipped",
			allowedOrigins: []string{"://invalid", "http://localhost:3000"},
			origin:         "http://localhost:3000",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub(tt.allowedOrigins)
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := hub.checkOrigin(req)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestHub_ClientCount tests client counting.
func TestHub_ClientCount(t *testing.T) {
	hub := NewHub(nil)
	assert.Equal(t, 0, hub.ClientCount())

	// Manually add a client
	hub.mu.Lock()
	hub.clients[&Client{}] = true
	hub.mu.Unlock()

	assert.Equal(t, 1, hub.ClientCount())

	// Add another
	hub.mu.Lock()
	hub.clients[&Client{}] = true
	hub.mu.Unlock()

	assert.Equal(t, 2, hub.ClientCount())
}

// TestHub_Broadcast tests broadcasting messages.
func TestHub_Broadcast(t *testing.T) {
	hub := NewHub(nil)

	// Broadcast with no clients should not panic
	hub.Broadcast(Message{Type: "test", Payload: "hello"})

	// Start hub in background
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	defer cancel()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Broadcast again
	hub.Broadcast(Message{Type: "test", Payload: map[string]string{"key": "value"}})

	// Should not panic
}

// TestHub_BroadcastAfterClose tests broadcasting after hub is closed.
func TestHub_BroadcastAfterClose(t *testing.T) {
	hub := NewHub(nil)
	hub.Close()

	// Broadcast after close should not panic
	hub.Broadcast(Message{Type: "test", Payload: "should not panic"})
}

// TestHub_Close tests hub close functionality.
func TestHub_Close(t *testing.T) {
	hub := NewHub(nil)

	// Add some mock clients
	client1 := &Client{send: make(chan []byte, 1)}
	client2 := &Client{send: make(chan []byte, 1)}

	hub.mu.Lock()
	hub.clients[client1] = true
	hub.clients[client2] = true
	hub.mu.Unlock()

	assert.Equal(t, 2, hub.ClientCount())

	// Close hub
	hub.Close()

	assert.Equal(t, 0, hub.ClientCount())
	assert.True(t, hub.closed)

	// Close again should not panic
	hub.Close()
}

// TestHub_Run tests the hub event loop.
func TestHub_Run(t *testing.T) {
	hub := NewHub([]string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Register a client
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client

	// Give registration time to process
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.ClientCount())

	// Unregister the client
	hub.unregister <- client

	// Give unregistration time to process
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.ClientCount())

	// Cancel context to stop hub
	cancel()

	// Give hub time to close
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	closed := hub.closed
	hub.mu.RUnlock()
	assert.True(t, closed)
}

// TestHub_RunWithBroadcast tests broadcasting through the event loop.
func TestHub_RunWithBroadcast(t *testing.T) {
	hub := NewHub([]string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Register a client
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client

	// Give registration time to process
	time.Sleep(10 * time.Millisecond)

	// Broadcast a message
	hub.Broadcast(Message{Type: "test", Payload: "hello"})

	// Client should receive the message
	select {
	case msg := <-client.send:
		assert.Contains(t, string(msg), "test")
		assert.Contains(t, string(msg), "hello")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("client did not receive message in time")
	}
}

// TestMessage tests message structure.
func TestMessage(t *testing.T) {
	msg := Message{
		Type:    "release.created",
		Payload: map[string]string{"id": "123"},
	}

	assert.Equal(t, "release.created", msg.Type)
	payload, ok := msg.Payload.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "123", payload["id"])
}
