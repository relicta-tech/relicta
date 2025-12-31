// Package websocket provides WebSocket connection management for real-time updates.
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message.
	writeWait = 10 * time.Second
	// pongWait is the time allowed to read a pong message.
	pongWait = 60 * time.Second
	// pingPeriod is the frequency of ping messages (must be < pongWait).
	pingPeriod = (pongWait * 9) / 10
	// maxMessageSize is the maximum message size allowed.
	maxMessageSize = 512 * 1024 // 512KB
)

// Message represents a WebSocket message.
type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Client represents a WebSocket client connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients        map[*Client]bool
	broadcast      chan []byte
	register       chan *Client
	unregister     chan *Client
	mu             sync.RWMutex
	closed         bool
	allowedOrigins []string
	upgrader       websocket.Upgrader
}

// NewHub creates a new WebSocket hub with optional allowed origins for CSWSH protection.
// If allowedOrigins is empty, only same-origin connections are allowed.
func NewHub(allowedOrigins []string) *Hub {
	h := &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan []byte, 256),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		allowedOrigins: allowedOrigins,
	}

	// Configure upgrader with origin validation
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	return h
}

// checkOrigin validates the Origin header against allowed origins.
// This prevents Cross-Site WebSocket Hijacking (CSWSH) attacks.
func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// No origin header means same-origin request (e.g., from same host)
	if origin == "" {
		return true
	}

	// If no origins are configured, reject all cross-origin requests
	if len(h.allowedOrigins) == 0 {
		slog.Debug("websocket origin rejected: no allowed origins configured", "origin", origin)
		return false
	}

	// Parse the origin
	originURL, err := url.Parse(origin)
	if err != nil {
		slog.Debug("websocket origin rejected: invalid origin URL", "origin", origin, "error", err)
		return false
	}

	// Check against allowed origins
	for _, allowed := range h.allowedOrigins {
		// Handle wildcard
		if allowed == "*" {
			return true
		}

		// Parse allowed origin for comparison
		allowedURL, err := url.Parse(allowed)
		if err != nil {
			continue
		}

		// Compare scheme and host (host includes port)
		if strings.EqualFold(originURL.Scheme, allowedURL.Scheme) &&
			strings.EqualFold(originURL.Host, allowedURL.Host) {
			return true
		}
	}

	slog.Debug("websocket origin rejected: not in allowed list", "origin", origin, "allowed", h.allowedOrigins)
	return false
}

// Run starts the hub event loop.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.close()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Debug("websocket client connected", "clients", len(h.clients))
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Debug("websocket client disconnected", "clients", len(h.clients))
		case message := <-h.broadcast:
			// Collect clients with full buffers to remove after iteration
			// This avoids lock thrashing (RLock → Lock → RLock pattern)
			var toRemove []*Client

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, mark for removal
					toRemove = append(toRemove, client)
				}
			}
			h.mu.RUnlock()

			// Remove slow clients in a separate phase with write lock
			if len(toRemove) > 0 {
				h.mu.Lock()
				for _, client := range toRemove {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Close closes all client connections.
func (h *Hub) Close() {
	h.close()
}

func (h *Hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	h.closed = true

	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal websocket message", "error", err)
		return
	}

	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	select {
	case h.broadcast <- data:
	default:
		slog.Warn("websocket broadcast channel full, dropping message")
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleConnection handles a WebSocket upgrade request.
func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Debug("websocket read error", "error", err)
			}
			break
		}
		// Currently we don't process incoming messages from clients
		// The dashboard is read-only for real-time updates
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				_ = w.Close()
				return
			}

			// Add queued messages to the current write
			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					_ = w.Close()
					return
				}
				if _, err := w.Write(<-c.send); err != nil {
					_ = w.Close()
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
