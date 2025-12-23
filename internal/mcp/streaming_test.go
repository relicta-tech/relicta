package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStreamingTransport implements StreamingTransport for testing.
type mockStreamingTransport struct {
	*StdioTransport
	notifications []struct {
		Method string
		Params any
	}
	mu sync.Mutex
}

func newMockStreamingTransport() *mockStreamingTransport {
	return &mockStreamingTransport{}
}

func (t *mockStreamingTransport) WriteNotification(method string, params any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifications = append(t.notifications, struct {
		Method string
		Params any
	}{Method: method, Params: params})
	return nil
}

func (t *mockStreamingTransport) WriteNotificationAsync(method string, params any) error {
	return t.WriteNotification(method, params)
}

func (t *mockStreamingTransport) ReadMessage() (*Request, error) {
	return nil, nil
}

func (t *mockStreamingTransport) WriteResponse(resp *Response) error {
	return nil
}

func (t *mockStreamingTransport) Close() error {
	return nil
}

func (t *mockStreamingTransport) getNotifications() []struct {
	Method string
	Params any
} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.notifications
}

func TestNewStreamReporter(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	assert.NotNil(t, reporter)
	assert.NotNil(t, reporter.tokens)
}

func TestStreamReporter_Start(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	token, err := reporter.Start(context.Background(), 10, "Processing commits")

	require.NoError(t, err)
	assert.NotEmpty(t, token)

	notifications := transport.getNotifications()
	require.Len(t, notifications, 1)
	assert.Equal(t, "notifications/progress", notifications[0].Method)

	progress := notifications[0].Params.(*Progress)
	assert.Equal(t, token, progress.Token)
	assert.Equal(t, "begin", progress.Value.Kind)
	assert.Equal(t, "Processing commits", progress.Value.Title)
	assert.Equal(t, 10, progress.TotalSteps)
	assert.Equal(t, float64(0), progress.Value.Percentage)
}

func TestStreamReporter_Update(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	token, err := reporter.Start(context.Background(), 10, "Processing")
	require.NoError(t, err)

	err = reporter.Update(context.Background(), token, 5, "Halfway done")
	require.NoError(t, err)

	notifications := transport.getNotifications()
	require.Len(t, notifications, 2)

	progress := notifications[1].Params.(*Progress)
	assert.Equal(t, token, progress.Token)
	assert.Equal(t, "report", progress.Value.Kind)
	assert.Equal(t, "Halfway done", progress.Value.Message)
	assert.Equal(t, float64(50), progress.Value.Percentage)
}

func TestStreamReporter_Complete(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	token, err := reporter.Start(context.Background(), 10, "Processing")
	require.NoError(t, err)

	err = reporter.Complete(context.Background(), token, "Completed successfully")
	require.NoError(t, err)

	notifications := transport.getNotifications()
	require.Len(t, notifications, 2)

	progress := notifications[1].Params.(*Progress)
	assert.Equal(t, token, progress.Token)
	assert.Equal(t, "end", progress.Value.Kind)
	assert.Equal(t, "Completed successfully", progress.Value.Message)
	assert.Equal(t, float64(100), progress.Value.Percentage)
}

func TestStreamReporter_UpdateUnknownToken(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	// Update with unknown token should not panic or error
	err := reporter.Update(context.Background(), "unknown-token", 5, "message")
	require.NoError(t, err)

	// No notifications should be sent
	notifications := transport.getNotifications()
	assert.Len(t, notifications, 0)
}

func TestStreamReporter_FullCycle(t *testing.T) {
	transport := newMockStreamingTransport()
	reporter := NewStreamReporter(transport)

	// Start
	token, err := reporter.Start(context.Background(), 4, "Analyzing commits")
	require.NoError(t, err)

	// Progress updates
	for i := 1; i <= 4; i++ {
		err = reporter.Update(context.Background(), token, i, "Processing commit "+string(rune('0'+i)))
		require.NoError(t, err)
	}

	// Complete
	err = reporter.Complete(context.Background(), token, "Analysis complete")
	require.NoError(t, err)

	notifications := transport.getNotifications()
	require.Len(t, notifications, 6) // 1 begin + 4 updates + 1 end

	// Verify progression
	assert.Equal(t, "begin", notifications[0].Params.(*Progress).Value.Kind)
	assert.Equal(t, float64(25), notifications[1].Params.(*Progress).Value.Percentage)
	assert.Equal(t, float64(50), notifications[2].Params.(*Progress).Value.Percentage)
	assert.Equal(t, float64(75), notifications[3].Params.(*Progress).Value.Percentage)
	assert.Equal(t, float64(100), notifications[4].Params.(*Progress).Value.Percentage)
	assert.Equal(t, "end", notifications[5].Params.(*Progress).Value.Kind)
}

func TestStreamLogger(t *testing.T) {
	transport := newMockStreamingTransport()
	logger := NewStreamLogger(transport, "test-logger")

	assert.NotNil(t, logger)
	assert.Equal(t, "test-logger", logger.logger)
}

func TestStreamLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(l *StreamLogger, ctx context.Context) error
		expected string
	}{
		{
			name: "debug",
			logFunc: func(l *StreamLogger, ctx context.Context) error {
				return l.Debug(ctx, "debug message", nil)
			},
			expected: LogLevelDebug,
		},
		{
			name: "info",
			logFunc: func(l *StreamLogger, ctx context.Context) error {
				return l.Info(ctx, "info message", nil)
			},
			expected: LogLevelInfo,
		},
		{
			name: "warning",
			logFunc: func(l *StreamLogger, ctx context.Context) error {
				return l.Warning(ctx, "warning message", nil)
			},
			expected: LogLevelWarning,
		},
		{
			name: "error",
			logFunc: func(l *StreamLogger, ctx context.Context) error {
				return l.Error(ctx, "error message", nil)
			},
			expected: LogLevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := newMockStreamingTransport()
			logger := NewStreamLogger(transport, "test")

			err := tt.logFunc(logger, context.Background())
			require.NoError(t, err)

			notifications := transport.getNotifications()
			require.Len(t, notifications, 1)
			assert.Equal(t, "notifications/message", notifications[0].Method)

			log := notifications[0].Params.(*LogNotification)
			assert.Equal(t, tt.expected, log.Level)
		})
	}
}

func TestStreamLogger_WithData(t *testing.T) {
	transport := newMockStreamingTransport()
	logger := NewStreamLogger(transport, "test")

	data := map[string]interface{}{
		"commit_count": 15,
		"files":        []string{"a.go", "b.go"},
	}

	err := logger.Info(context.Background(), "Processing complete", data)
	require.NoError(t, err)

	notifications := transport.getNotifications()
	require.Len(t, notifications, 1)

	log := notifications[0].Params.(*LogNotification)
	assert.Equal(t, "Processing complete", log.Message)
	assert.Equal(t, "test", log.Logger)
	assert.NotNil(t, log.Data)
}

func TestNewStreamingServer(t *testing.T) {
	server, err := NewStreamingServer("1.0.0")

	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.Server)
}

func TestStreamingServer_SetTransport(t *testing.T) {
	server, err := NewStreamingServer("1.0.0")
	require.NoError(t, err)

	transport := newMockStreamingTransport()
	server.SetStreamingTransport(transport)

	assert.NotNil(t, server.Reporter())
	assert.NotNil(t, server.Logger())
}

func TestStreamingStdioTransport(t *testing.T) {
	base := &StdioTransport{}
	streaming := NewStreamingStdioTransport(base)

	assert.NotNil(t, streaming)
	assert.Equal(t, base, streaming.StdioTransport)
}

func TestNewStreamingClient(t *testing.T) {
	transport := newMockTransport()
	client := NewStreamingClient(transport)

	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
	assert.NotNil(t, client.progressCallbacks)
}

func TestStreamingClient_ProgressCallbacks(t *testing.T) {
	transport := newMockTransport()
	client := NewStreamingClient(transport)

	var received *Progress
	token := ProgressToken("test-token")

	client.OnProgress(token, func(p *Progress) {
		received = p
	})

	// Simulate notification
	progress := Progress{
		Token: token,
		Value: ProgressValue{
			Kind:       "report",
			Message:    "50% done",
			Percentage: 50,
		},
	}
	params, _ := json.Marshal(progress)
	client.HandleNotification("notifications/progress", params)

	require.NotNil(t, received)
	assert.Equal(t, token, received.Token)
	assert.Equal(t, float64(50), received.Value.Percentage)
}

func TestStreamingClient_RemoveProgressCallback(t *testing.T) {
	transport := newMockTransport()
	client := NewStreamingClient(transport)

	var callCount int
	token := ProgressToken("test-token")

	client.OnProgress(token, func(p *Progress) {
		callCount++
	})

	// First notification
	progress := Progress{Token: token}
	params, _ := json.Marshal(progress)
	client.HandleNotification("notifications/progress", params)
	assert.Equal(t, 1, callCount)

	// Remove callback
	client.RemoveProgressCallback(token)

	// Second notification (should not call callback)
	client.HandleNotification("notifications/progress", params)
	assert.Equal(t, 1, callCount)
}

func TestStreamingClient_HandleUnknownNotification(t *testing.T) {
	transport := newMockTransport()
	client := NewStreamingClient(transport)

	// Should not panic
	client.HandleNotification("unknown/method", nil)
}

func TestStreamingClient_HandleInvalidProgress(t *testing.T) {
	transport := newMockTransport()
	client := NewStreamingClient(transport)

	// Should not panic with invalid JSON
	client.HandleNotification("notifications/progress", []byte("invalid"))
}

func TestStreamingClient_PlanWithProgress(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"release_id": "rel-123",
			"current_version": "1.0.0",
			"suggested_bump": "minor",
			"commit_count": 10,
			"features": 2,
			"fixes": 3
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewStreamingClient(transport)

	var progressMessages []string
	planResult, err := client.PlanWithProgress(
		context.Background(),
		true,
		"v1.0.0",
		func(message string, percentage float64) {
			progressMessages = append(progressMessages, message)
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "rel-123", planResult.ReleaseID)
	assert.Equal(t, "minor", planResult.SuggestedBump)
}

func TestStreamingClient_PlanWithProgress_Error(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: "No commits found"}},
		IsError: true,
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewStreamingClient(transport)

	_, err := client.PlanWithProgress(context.Background(), false, "", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "No commits found")
}

func TestStreamingClient_PublishWithProgress(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"published": true,
			"version": "1.1.0",
			"tag": "v1.1.0"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewStreamingClient(transport)

	var progressMessages []string
	publishResult, err := client.PublishWithProgress(
		context.Background(),
		"rel-123",
		false,
		func(message string, percentage float64) {
			progressMessages = append(progressMessages, message)
		},
	)

	require.NoError(t, err)
	assert.True(t, publishResult.Published)
	assert.Equal(t, "v1.1.0", publishResult.Tag)
}

func TestStreamingClient_PublishWithProgress_Error(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: "Not approved"}},
		IsError: true,
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewStreamingClient(transport)

	_, err := client.PublishWithProgress(context.Background(), "", false, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not approved")
}

func TestProgress_JSON(t *testing.T) {
	progress := &Progress{
		Token:      "test-token",
		TotalSteps: 10,
		Value: ProgressValue{
			Kind:        "report",
			Message:     "Processing",
			Percentage:  50.5,
			Cancellable: true,
		},
	}

	data, err := json.Marshal(progress)
	require.NoError(t, err)

	var decoded Progress
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, ProgressToken("test-token"), decoded.Token)
	assert.Equal(t, 10, decoded.TotalSteps)
	assert.Equal(t, "report", decoded.Value.Kind)
	assert.Equal(t, 50.5, decoded.Value.Percentage)
	assert.True(t, decoded.Value.Cancellable)
}

func TestLogNotification_JSON(t *testing.T) {
	log := &LogNotification{
		Level:   LogLevelInfo,
		Message: "Test message",
		Logger:  "test-logger",
		Data: map[string]int{
			"count": 42,
		},
	}

	data, err := json.Marshal(log)
	require.NoError(t, err)

	var decoded LogNotification
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, LogLevelInfo, decoded.Level)
	assert.Equal(t, "Test message", decoded.Message)
	assert.Equal(t, "test-logger", decoded.Logger)
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken(1)
	token2 := generateToken(2)

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2)
}
