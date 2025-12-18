package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitialize(t *testing.T) {
	t.Run("empty path disables logging", func(t *testing.T) {
		err := Initialize("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer Close()

		if IsEnabled() {
			t.Error("expected logging to be disabled with empty path")
		}
	})

	t.Run("valid path enables logging", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		err := Initialize(logPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer Close()

		if !IsEnabled() {
			t.Error("expected logging to be enabled")
		}

		// Verify file was created
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("expected log file to be created")
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		err := Initialize("/nonexistent/directory/audit.log")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

func TestLog(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()

	event := Event{
		Timestamp:  time.Now().UTC(),
		PluginName: "test-plugin",
		Hook:       "pre-release",
		EventType:  EventTypeExecute,
		Success:    true,
		Duration:   150 * time.Millisecond,
	}

	err = Log(ctx, event)
	if err != nil {
		t.Fatalf("failed to log event: %v", err)
	}

	// Close to flush
	Close()

	// Read and verify log content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test-plugin") {
		t.Error("expected log to contain plugin name")
	}
	if !strings.Contains(string(content), "pre-release") {
		t.Error("expected log to contain hook name")
	}
	if !strings.Contains(string(content), "execute") {
		t.Error("expected log to contain event type")
	}
}

func TestLogLoad(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()

	err = LogLoad(ctx, "github-plugin", true, "")
	if err != nil {
		t.Fatalf("failed to log load event: %v", err)
	}

	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "github-plugin") {
		t.Error("expected log to contain plugin name")
	}
	if !strings.Contains(string(content), `"event_type":"load"`) {
		t.Error("expected log to contain load event type")
	}
	if !strings.Contains(string(content), `"success":true`) {
		t.Error("expected log to contain success:true")
	}
}

func TestLogExecution(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()

	err = LogExecution(ctx, "slack-plugin", "post-release", true, 250*time.Millisecond, "")
	if err != nil {
		t.Fatalf("failed to log execution event: %v", err)
	}

	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "slack-plugin") {
		t.Error("expected log to contain plugin name")
	}
	if !strings.Contains(string(content), "post-release") {
		t.Error("expected log to contain hook name")
	}
	if !strings.Contains(string(content), `"duration_ms":250`) {
		t.Error("expected log to contain duration in milliseconds")
	}
}

func TestLogTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()

	err = LogTimeout(ctx, "slow-plugin", "pre-version", 30*time.Second)
	if err != nil {
		t.Fatalf("failed to log timeout event: %v", err)
	}

	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), `"event_type":"timeout"`) {
		t.Error("expected log to contain timeout event type")
	}
	if !strings.Contains(string(content), "operation timed out") {
		t.Error("expected log to contain timeout error message")
	}
}

func TestLogRejected(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()
	metadata := map[string]any{
		"attempted_path": "/etc/passwd",
		"allowed_paths":  []string{"/tmp"},
	}

	err = LogRejected(ctx, "malicious-plugin", "filesystem access denied", metadata)
	if err != nil {
		t.Fatalf("failed to log rejected event: %v", err)
	}

	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), `"event_type":"rejected"`) {
		t.Error("expected log to contain rejected event type")
	}
	if !strings.Contains(string(content), "filesystem access denied") {
		t.Error("expected log to contain rejection reason")
	}
	if !strings.Contains(string(content), "/etc/passwd") {
		t.Error("expected log to contain metadata")
	}
}

func TestLogWhenDisabled(t *testing.T) {
	// Initialize with empty path (disabled)
	err := Initialize("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer Close()

	ctx := context.Background()

	// These should all succeed silently when disabled
	err = LogLoad(ctx, "plugin", true, "")
	if err != nil {
		t.Errorf("LogLoad should not error when disabled: %v", err)
	}

	err = LogExecution(ctx, "plugin", "hook", true, time.Second, "")
	if err != nil {
		t.Errorf("LogExecution should not error when disabled: %v", err)
	}

	err = LogTimeout(ctx, "plugin", "hook", time.Second)
	if err != nil {
		t.Errorf("LogTimeout should not error when disabled: %v", err)
	}
}

func TestNewLogger(t *testing.T) {
	t.Run("empty path creates disabled logger", func(t *testing.T) {
		logger, err := NewLogger("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer logger.Close()

		if logger.Enabled() {
			t.Error("expected logger to be disabled")
		}
		if logger.Path() != "" {
			t.Error("expected empty path")
		}
	})

	t.Run("valid path creates enabled logger", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		logger, err := NewLogger(logPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer logger.Close()

		if !logger.Enabled() {
			t.Error("expected logger to be enabled")
		}
		if logger.Path() != logPath {
			t.Errorf("expected path %s, got %s", logPath, logger.Path())
		}
	})
}

func TestLoggerLogEvent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()

	event := Event{
		Timestamp:  time.Now().UTC(),
		PluginName: "instance-test",
		EventType:  EventTypeLoad,
		Success:    true,
		Duration:   50 * time.Millisecond,
	}

	err = logger.LogEvent(ctx, event)
	if err != nil {
		t.Fatalf("failed to log event: %v", err)
	}

	logger.Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "instance-test") {
		t.Error("expected log to contain plugin name")
	}
}

func TestEventMarshalJSON(t *testing.T) {
	event := Event{
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		PluginName: "json-test",
		Hook:       "pre-release",
		EventType:  EventTypeExecute,
		Success:    true,
		Duration:   1500 * time.Millisecond,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Duration should be in milliseconds
	durationMS, ok := result["duration_ms"].(float64)
	if !ok {
		t.Fatal("expected duration_ms to be a number")
	}
	if durationMS != 1500 {
		t.Errorf("expected duration_ms to be 1500, got %v", durationMS)
	}
}

func TestClose(t *testing.T) {
	t.Run("close when not initialized", func(t *testing.T) {
		// Reset global state
		globalMu.Lock()
		globalLogger = nil
		globalMu.Unlock()

		err := Close()
		if err != nil {
			t.Errorf("Close should not error when not initialized: %v", err)
		}
	})

	t.Run("close when disabled", func(t *testing.T) {
		Initialize("")
		err := Close()
		if err != nil {
			t.Errorf("Close should not error when disabled: %v", err)
		}
	})
}

func TestNilLoggerMethods(t *testing.T) {
	var logger *Logger

	// All methods should handle nil gracefully
	err := logger.LogEvent(context.Background(), Event{})
	if err != nil {
		t.Errorf("LogEvent should not error on nil logger: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("Close should not error on nil logger: %v", err)
	}

	if logger.Path() != "" {
		t.Error("Path should return empty string for nil logger")
	}

	if logger.Enabled() {
		t.Error("Enabled should return false for nil logger")
	}
}

func TestConcurrentLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	err := Initialize(logPath)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}
	defer Close()

	ctx := context.Background()
	done := make(chan bool)

	// Spawn multiple goroutines logging concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				LogExecution(ctx, "concurrent-plugin", "hook", true, time.Millisecond, "")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	Close()

	// Verify file was written
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Should have 100 log entries
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 log entries, got %d", len(lines))
	}
}
