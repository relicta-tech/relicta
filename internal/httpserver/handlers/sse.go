package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	ID    uint64 `json:"id"`
	Type  string `json:"type"`
	Data  any    `json:"data"`
	Retry int    `json:"retry,omitempty"` // milliseconds
}

// SSEHub manages SSE client connections and broadcasts events.
// It provides a fallback for environments where WebSocket is not available.
type SSEHub struct {
	mu          sync.RWMutex
	clients     map[uint64]chan SSEEvent
	nextID      atomic.Uint64
	eventID     atomic.Uint64
	replayBuf   []SSEEvent
	replayLimit int
	closed      bool
}

// NewSSEHub creates a new SSE hub.
// replayLimit controls how many recent events are kept for Last-Event-ID reconnection.
func NewSSEHub(replayLimit int) *SSEHub {
	if replayLimit <= 0 {
		replayLimit = 256
	}
	return &SSEHub{
		clients:     make(map[uint64]chan SSEEvent),
		replayBuf:   make([]SSEEvent, 0, replayLimit),
		replayLimit: replayLimit,
	}
}

// Broadcast sends an event to all connected SSE clients and stores it for replay.
func (h *SSEHub) Broadcast(eventType string, data any) {
	id := h.eventID.Add(1)
	evt := SSEEvent{
		ID:   id,
		Type: eventType,
		Data: data,
	}

	h.mu.Lock()
	// Store in replay buffer (ring buffer behavior)
	if len(h.replayBuf) >= h.replayLimit {
		h.replayBuf = h.replayBuf[1:]
	}
	h.replayBuf = append(h.replayBuf, evt)
	h.mu.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for _, ch := range h.clients {
		select {
		case ch <- evt:
		default:
			// Client too slow, skip
		}
	}
}

// addClient registers a new SSE client and returns a channel and a cleanup function.
func (h *SSEHub) addClient() (uint64, <-chan SSEEvent, func()) {
	id := h.nextID.Add(1)
	ch := make(chan SSEEvent, 64)
	h.mu.Lock()
	h.clients[id] = ch
	h.mu.Unlock()
	return id, ch, func() {
		h.mu.Lock()
		delete(h.clients, id)
		h.mu.Unlock()
	}
}

// replaySince returns events with IDs greater than lastID.
func (h *SSEHub) replaySince(lastID uint64) []SSEEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []SSEEvent
	for _, evt := range h.replayBuf {
		if evt.ID > lastID {
			result = append(result, evt)
		}
	}
	return result
}

// Close shuts down the SSE hub and closes all client channels.
func (h *SSEHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, ch := range h.clients {
		close(ch)
		delete(h.clients, id)
	}
}

// ClientCount returns the number of connected SSE clients.
func (h *SSEHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SSEStreamHandler returns an HTTP handler that streams Server-Sent Events.
// It supports the Last-Event-ID header for reconnection.
func SSEStreamHandler(hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

		// Register client
		_, events, cleanup := hub.addClient()
		defer cleanup()

		// Replay missed events if Last-Event-ID is provided
		if lastIDStr := r.Header.Get("Last-Event-ID"); lastIDStr != "" {
			lastID, err := strconv.ParseUint(lastIDStr, 10, 64)
			if err == nil {
				for _, evt := range hub.replaySince(lastID) {
					if err := writeSSEEvent(w, evt); err != nil {
						slog.Debug("sse replay write failed", "error", err)
						return
					}
				}
				flusher.Flush()
			}
		}

		// Send initial retry hint (5 seconds)
		fmt.Fprintf(w, "retry: 5000\n\n")
		flusher.Flush()

		// Stream events until client disconnects
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				if err := writeSSEEvent(w, evt); err != nil {
					slog.Debug("sse write failed", "error", err)
					return
				}
				flusher.Flush()
			}
		}
	}
}

// writeSSEEvent writes a single SSE event to the writer.
func writeSSEEvent(w http.ResponseWriter, evt SSEEvent) error {
	data, err := json.Marshal(evt.Data)
	if err != nil {
		return fmt.Errorf("marshal sse data: %w", err)
	}

	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", evt.ID, evt.Type, data)
	return err
}
