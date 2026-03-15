package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DeliveryLogEntry represents a single delivery attempt in the log.
type DeliveryLogEntry struct {
	// DeliveryID links to the Delivery.
	DeliveryID string `json:"delivery_id"`
	// Timestamp is when the attempt was made.
	Timestamp time.Time `json:"timestamp"`
	// EndpointURL is the target webhook URL.
	EndpointURL string `json:"endpoint_url"`
	// StatusCode is the HTTP response status code (0 if connection failed).
	StatusCode int `json:"status_code"`
	// ResponseTimeMS is the request duration in milliseconds.
	ResponseTimeMS int64 `json:"response_time_ms"`
	// PayloadHash is the SHA-256 hash of the delivered payload.
	PayloadHash string `json:"payload_hash"`
	// Success indicates whether the delivery was successful.
	Success bool `json:"success"`
	// Error contains the error message if the delivery failed.
	Error string `json:"error,omitempty"`
	// Attempt is the attempt number (1-based).
	Attempt int `json:"attempt"`
	// Event is the webhook event name.
	Event string `json:"event,omitempty"`
}

// DeliveryLog stores delivery attempt logs on disk, one file per endpoint.
// Each file contains a JSON array of DeliveryLogEntry.
type DeliveryLog struct {
	baseDir string
	mu      sync.RWMutex
}

// NewDeliveryLog creates a new file-based delivery log.
func NewDeliveryLog(baseDir string) (*DeliveryLog, error) {
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	return &DeliveryLog{baseDir: logDir}, nil
}

// Record appends a delivery attempt entry to the endpoint's log file.
func (l *DeliveryLog) Record(entry DeliveryLogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	path := l.endpointLogPath(entry.EndpointURL)

	entries, err := l.readLogFile(path)
	if err != nil {
		entries = nil
	}

	entries = append(entries, entry)

	return l.writeLogFile(path, entries)
}

// List returns all delivery log entries for a given endpoint URL,
// sorted by timestamp (newest first).
func (l *DeliveryLog) List(endpointURL string) ([]DeliveryLogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	path := l.endpointLogPath(endpointURL)
	entries, err := l.readLogFile(path)
	if err != nil {
		return nil, nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	return entries, nil
}

// ListByDeliveryID returns all log entries for a specific delivery ID.
func (l *DeliveryLog) ListByDeliveryID(endpointURL, deliveryID string) ([]DeliveryLogEntry, error) {
	entries, err := l.List(endpointURL)
	if err != nil {
		return nil, err
	}

	var filtered []DeliveryLogEntry
	for _, e := range entries {
		if e.DeliveryID == deliveryID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// endpointLogPath returns the file path for an endpoint's log.
// The endpoint URL is hashed to create a safe filename.
func (l *DeliveryLog) endpointLogPath(endpointURL string) string {
	// Sanitize URL to create a safe filename
	safe := strings.NewReplacer(
		"://", "_",
		"/", "_",
		":", "_",
		"?", "_",
		"&", "_",
		"=", "_",
	).Replace(endpointURL)

	// Truncate if too long
	if len(safe) > 100 {
		safe = safe[:100]
	}

	return filepath.Join(l.baseDir, safe+".json")
}

// readLogFile reads and parses a log file.
func (l *DeliveryLog) readLogFile(path string) ([]DeliveryLogEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []DeliveryLogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse log file %s: %w", path, err)
	}
	return entries, nil
}

// writeLogFile writes entries to a log file.
func (l *DeliveryLog) writeLogFile(path string, entries []DeliveryLogEntry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal log entries: %w", err)
	}
	return os.WriteFile(path, data, 0o640)
}
