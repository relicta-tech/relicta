// Package audit provides audit logging for plugin operations.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventType represents the type of plugin event being logged.
type EventType string

const (
	EventTypeLoad     EventType = "load"
	EventTypeUnload   EventType = "unload"
	EventTypeExecute  EventType = "execute"
	EventTypeError    EventType = "error"
	EventTypeTimeout  EventType = "timeout"
	EventTypeRejected EventType = "rejected"
)

// Event represents a single audit log entry for a plugin operation.
type Event struct {
	Timestamp    time.Time      `json:"timestamp"`
	PluginName   string         `json:"plugin_name"`
	Hook         string         `json:"hook,omitempty"`
	EventType    EventType      `json:"event_type"`
	Success      bool           `json:"success"`
	Duration     time.Duration  `json:"duration_ms"`
	ErrorMessage string         `json:"error,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON provides custom JSON marshaling with duration in milliseconds.
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(&struct {
		Alias
		Duration int64 `json:"duration_ms"`
	}{
		Alias:    Alias(e),
		Duration: e.Duration.Milliseconds(),
	})
}

// Logger provides audit logging capabilities for plugin operations.
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	encoder  *json.Encoder
	enabled  bool
}

// globalLogger is the package-level logger instance.
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// Initialize sets up the global audit logger with the specified log file path.
// If path is empty, audit logging is disabled.
func Initialize(path string) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	// Close existing logger if any
	if globalLogger != nil && globalLogger.file != nil {
		globalLogger.file.Close()
	}

	if path == "" {
		globalLogger = &Logger{enabled: false}
		return nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}

	globalLogger = &Logger{
		file:     file,
		filePath: path,
		encoder:  json.NewEncoder(file),
		enabled:  true,
	}

	return nil
}

// Close closes the global audit logger.
func Close() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalLogger == nil || globalLogger.file == nil {
		return nil
	}

	err := globalLogger.file.Close()
	globalLogger = nil
	return err
}

// IsEnabled returns whether audit logging is currently enabled.
func IsEnabled() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalLogger != nil && globalLogger.enabled
}

// Log writes an audit event to the log file.
func Log(_ context.Context, event Event) error {
	globalMu.RLock()
	logger := globalLogger
	globalMu.RUnlock()

	if logger == nil || !logger.enabled {
		return nil
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()

	if err := logger.encoder.Encode(event); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// LogLoad logs a plugin load event.
// The errorMsg parameter should be empty string for success, or the error message for failures.
func LogLoad(ctx context.Context, pluginName string, success bool, errorMsg string) error {
	event := Event{
		Timestamp:    time.Now().UTC(),
		PluginName:   pluginName,
		EventType:    EventTypeLoad,
		Success:      success,
		ErrorMessage: errorMsg,
	}
	return Log(ctx, event)
}

// LogUnload logs a plugin unload event.
func LogUnload(ctx context.Context, pluginName string) error {
	event := Event{
		Timestamp:  time.Now().UTC(),
		PluginName: pluginName,
		EventType:  EventTypeUnload,
		Success:    true,
	}
	return Log(ctx, event)
}

// LogExecution logs a plugin hook execution event.
// The errorMsg parameter should be empty string for success, or the error message for failures.
func LogExecution(ctx context.Context, pluginName, hook string, success bool, duration time.Duration, errorMsg string) error {
	event := Event{
		Timestamp:    time.Now().UTC(),
		PluginName:   pluginName,
		Hook:         hook,
		EventType:    EventTypeExecute,
		Success:      success,
		Duration:     duration,
		ErrorMessage: errorMsg,
	}
	return Log(ctx, event)
}

// LogTimeout logs a plugin timeout event.
func LogTimeout(ctx context.Context, pluginName, hook string, duration time.Duration) error {
	return Log(ctx, Event{
		Timestamp:    time.Now().UTC(),
		PluginName:   pluginName,
		Hook:         hook,
		EventType:    EventTypeTimeout,
		Success:      false,
		Duration:     duration,
		ErrorMessage: "operation timed out",
	})
}

// LogRejected logs when a plugin operation is rejected (e.g., sandbox violation).
func LogRejected(ctx context.Context, pluginName string, reason string, metadata map[string]any) error {
	return Log(ctx, Event{
		Timestamp:    time.Now().UTC(),
		PluginName:   pluginName,
		EventType:    EventTypeRejected,
		Success:      false,
		ErrorMessage: reason,
		Metadata:     metadata,
	})
}

// NewLogger creates a new Logger instance for direct use (not global).
func NewLogger(path string) (*Logger, error) {
	if path == "" {
		return &Logger{enabled: false}, nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &Logger{
		file:     file,
		filePath: path,
		encoder:  json.NewEncoder(file),
		enabled:  true,
	}, nil
}

// LogEvent writes an event to this logger instance.
func (l *Logger) LogEvent(_ context.Context, event Event) error {
	if l == nil || !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.encoder.Encode(event); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// Close closes this logger instance.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.file.Close()
}

// Path returns the file path of the audit log.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.filePath
}

// Enabled returns whether this logger is enabled.
func (l *Logger) Enabled() bool {
	if l == nil {
		return false
	}
	return l.enabled
}
