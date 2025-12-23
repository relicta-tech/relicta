package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// ProgressReporter sends progress updates during long-running operations.
type ProgressReporter interface {
	// ReportProgress sends a progress notification.
	ReportProgress(ctx context.Context, progress *Progress) error
	// Start begins a progress tracking session.
	Start(ctx context.Context, total int, message string) (ProgressToken, error)
	// Update sends an incremental progress update.
	Update(ctx context.Context, token ProgressToken, current int, message string) error
	// Complete marks the progress as finished.
	Complete(ctx context.Context, token ProgressToken, message string) error
}

// ProgressToken identifies a progress tracking session.
type ProgressToken string

// Progress represents a progress notification.
type Progress struct {
	Token      ProgressToken `json:"token"`
	Value      ProgressValue `json:"value"`
	TotalSteps int           `json:"totalSteps,omitempty"`
}

// ProgressValue contains the progress details.
type ProgressValue struct {
	Kind        string  `json:"kind"`                  // "begin", "report", "end"
	Title       string  `json:"title,omitempty"`       // For "begin"
	Message     string  `json:"message,omitempty"`     // Status message
	Percentage  float64 `json:"percentage,omitempty"`  // 0-100
	Cancellable bool    `json:"cancellable,omitempty"` // Whether operation can be canceled
}

// StreamingTransport extends Transport with notification capabilities.
type StreamingTransport interface {
	Transport
	// WriteNotificationAsync sends a notification without blocking.
	WriteNotificationAsync(method string, params any) error
}

// StreamReporter implements ProgressReporter over a transport.
type StreamReporter struct {
	transport StreamingTransport
	mu        sync.Mutex
	tokens    map[ProgressToken]*progressState
	nextID    int64
}

type progressState struct {
	total   int
	current int
	started time.Time
}

// NewStreamReporter creates a new stream reporter.
func NewStreamReporter(transport StreamingTransport) *StreamReporter {
	return &StreamReporter{
		transport: transport,
		tokens:    make(map[ProgressToken]*progressState),
	}
}

// ReportProgress sends a raw progress notification.
func (r *StreamReporter) ReportProgress(ctx context.Context, progress *Progress) error {
	return r.transport.WriteNotification("notifications/progress", progress)
}

// Start begins a progress tracking session.
func (r *StreamReporter) Start(ctx context.Context, total int, message string) (ProgressToken, error) {
	r.mu.Lock()
	r.nextID++
	token := ProgressToken(generateToken(r.nextID))
	r.tokens[token] = &progressState{
		total:   total,
		current: 0,
		started: time.Now(),
	}
	r.mu.Unlock()

	progress := &Progress{
		Token:      token,
		TotalSteps: total,
		Value: ProgressValue{
			Kind:        "begin",
			Title:       message,
			Percentage:  0,
			Cancellable: false,
		},
	}

	if err := r.ReportProgress(ctx, progress); err != nil {
		return "", err
	}

	return token, nil
}

// Update sends an incremental progress update.
func (r *StreamReporter) Update(ctx context.Context, token ProgressToken, current int, message string) error {
	r.mu.Lock()
	state, ok := r.tokens[token]
	if !ok {
		r.mu.Unlock()
		return nil // Token not found, ignore
	}
	state.current = current
	total := state.total
	r.mu.Unlock()

	var percentage float64
	if total > 0 {
		percentage = float64(current) / float64(total) * 100
	}

	progress := &Progress{
		Token: token,
		Value: ProgressValue{
			Kind:       "report",
			Message:    message,
			Percentage: percentage,
		},
	}

	return r.ReportProgress(ctx, progress)
}

// Complete marks the progress as finished.
func (r *StreamReporter) Complete(ctx context.Context, token ProgressToken, message string) error {
	r.mu.Lock()
	delete(r.tokens, token)
	r.mu.Unlock()

	progress := &Progress{
		Token: token,
		Value: ProgressValue{
			Kind:       "end",
			Message:    message,
			Percentage: 100,
		},
	}

	return r.ReportProgress(ctx, progress)
}

func generateToken(id int64) string {
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+id%26))
}

// LogNotification represents a logging notification.
type LogNotification struct {
	Level   string `json:"level"`   // "debug", "info", "warning", "error"
	Message string `json:"message"` // Log message
	Logger  string `json:"logger"`  // Logger name
	Data    any    `json:"data,omitempty"`
}

// LogLevel constants
const (
	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
)

// StreamLogger sends log notifications over the transport.
type StreamLogger struct {
	transport StreamingTransport
	logger    string
}

// NewStreamLogger creates a new stream logger.
func NewStreamLogger(transport StreamingTransport, logger string) *StreamLogger {
	return &StreamLogger{
		transport: transport,
		logger:    logger,
	}
}

// Log sends a log notification.
func (l *StreamLogger) Log(ctx context.Context, level string, message string, data any) error {
	notification := &LogNotification{
		Level:   level,
		Message: message,
		Logger:  l.logger,
		Data:    data,
	}
	return l.transport.WriteNotification("notifications/message", notification)
}

// Debug sends a debug log notification.
func (l *StreamLogger) Debug(ctx context.Context, message string, data any) error {
	return l.Log(ctx, LogLevelDebug, message, data)
}

// Info sends an info log notification.
func (l *StreamLogger) Info(ctx context.Context, message string, data any) error {
	return l.Log(ctx, LogLevelInfo, message, data)
}

// Warning sends a warning log notification.
func (l *StreamLogger) Warning(ctx context.Context, message string, data any) error {
	return l.Log(ctx, LogLevelWarning, message, data)
}

// Error sends an error log notification.
func (l *StreamLogger) Error(ctx context.Context, message string, data any) error {
	return l.Log(ctx, LogLevelError, message, data)
}

// StreamingServer extends Server with streaming capabilities.
type StreamingServer struct {
	*Server
	reporter *StreamReporter
	logger   *StreamLogger
}

// NewStreamingServer creates a server with streaming support.
func NewStreamingServer(version string, opts ...ServerOption) (*StreamingServer, error) {
	server, err := NewServer(version, opts...)
	if err != nil {
		return nil, err
	}

	return &StreamingServer{
		Server: server,
	}, nil
}

// SetStreamingTransport sets the transport for streaming notifications.
func (s *StreamingServer) SetStreamingTransport(transport StreamingTransport) {
	s.reporter = NewStreamReporter(transport)
	s.logger = NewStreamLogger(transport, "relicta")
}

// Reporter returns the progress reporter.
func (s *StreamingServer) Reporter() *StreamReporter {
	return s.reporter
}

// Logger returns the stream logger.
func (s *StreamingServer) Logger() *StreamLogger {
	return s.logger
}

// StreamingStdioTransport extends StdioTransport with async notification support.
type StreamingStdioTransport struct {
	*StdioTransport
}

// NewStreamingStdioTransport creates a new streaming stdio transport.
func NewStreamingStdioTransport(base *StdioTransport) *StreamingStdioTransport {
	return &StreamingStdioTransport{StdioTransport: base}
}

// WriteNotificationAsync sends a notification without blocking.
// For stdio, this is the same as WriteNotification since writes are buffered.
func (t *StreamingStdioTransport) WriteNotificationAsync(method string, params any) error {
	return t.WriteNotification(method, params)
}

// ProgressCallback is called when progress updates are received.
type ProgressCallback func(progress *Progress)

// StreamingClient extends Client with streaming support.
type StreamingClient struct {
	*Client
	progressCallbacks map[ProgressToken]ProgressCallback
	progressMu        sync.RWMutex
}

// NewStreamingClient creates a client with streaming support.
func NewStreamingClient(transport ClientTransport, opts ...ClientOption) *StreamingClient {
	return &StreamingClient{
		Client:            NewClient(transport, opts...),
		progressCallbacks: make(map[ProgressToken]ProgressCallback),
	}
}

// OnProgress registers a callback for progress updates.
func (c *StreamingClient) OnProgress(token ProgressToken, callback ProgressCallback) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	c.progressCallbacks[token] = callback
}

// RemoveProgressCallback removes a progress callback.
func (c *StreamingClient) RemoveProgressCallback(token ProgressToken) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	delete(c.progressCallbacks, token)
}

// HandleNotification processes incoming notifications.
func (c *StreamingClient) HandleNotification(method string, params json.RawMessage) {
	switch method {
	case "notifications/progress":
		var progress Progress
		if err := json.Unmarshal(params, &progress); err != nil {
			return
		}

		c.progressMu.RLock()
		callback, ok := c.progressCallbacks[progress.Token]
		c.progressMu.RUnlock()

		if ok {
			callback(&progress)
		}
	}
}

// StreamingToolOptions configures streaming behavior for tool calls.
type StreamingToolOptions struct {
	OnProgress func(progress *Progress)
	OnLog      func(log *LogNotification)
}

// CallToolWithProgress calls a tool with progress updates.
// This method handles both the tool result and streaming notifications.
func (c *StreamingClient) CallToolWithProgress(
	ctx context.Context,
	name string,
	args map[string]any,
	opts *StreamingToolOptions,
) (*CallToolResult, error) {
	// For streaming, we need a transport that can receive notifications
	// This is a simplified implementation - full implementation would
	// require bidirectional transport handling

	// Register progress callback if provided
	if opts != nil && opts.OnProgress != nil {
		// Generate a token for this call
		token := ProgressToken(generateToken(time.Now().UnixNano()))
		c.OnProgress(token, opts.OnProgress)
		defer c.RemoveProgressCallback(token)
	}

	return c.CallTool(ctx, name, args)
}

// PlanWithProgress calls the plan tool with progress updates.
func (c *StreamingClient) PlanWithProgress(
	ctx context.Context,
	analyze bool,
	from string,
	onProgress func(message string, percentage float64),
) (*PlanResult, error) {
	args := map[string]any{}
	if analyze {
		args["analyze"] = true
	}
	if from != "" {
		args["from"] = from
	}

	opts := &StreamingToolOptions{}
	if onProgress != nil {
		opts.OnProgress = func(p *Progress) {
			onProgress(p.Value.Message, p.Value.Percentage)
		}
	}

	result, err := c.CallToolWithProgress(ctx, "relicta.plan", args, opts)
	if err != nil {
		return nil, err
	}

	if result.IsError {
		if len(result.Content) > 0 {
			return nil, &ToolError{Message: result.Content[0].Text}
		}
		return nil, &ToolError{Message: "plan failed"}
	}

	if len(result.Content) == 0 {
		return nil, &ToolError{Message: "no result content"}
	}

	var planResult PlanResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &planResult); err != nil {
		return nil, err
	}

	return &planResult, nil
}

// PublishWithProgress calls the publish tool with progress updates.
func (c *StreamingClient) PublishWithProgress(
	ctx context.Context,
	releaseID string,
	skipPush bool,
	onProgress func(message string, percentage float64),
) (*PublishResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}
	if skipPush {
		args["skip_push"] = true
	}

	opts := &StreamingToolOptions{}
	if onProgress != nil {
		opts.OnProgress = func(p *Progress) {
			onProgress(p.Value.Message, p.Value.Percentage)
		}
	}

	result, err := c.CallToolWithProgress(ctx, "relicta.publish", args, opts)
	if err != nil {
		return nil, err
	}

	if result.IsError {
		if len(result.Content) > 0 {
			return nil, &ToolError{Message: result.Content[0].Text}
		}
		return nil, &ToolError{Message: "publish failed"}
	}

	if len(result.Content) == 0 {
		return nil, &ToolError{Message: "no result content"}
	}

	var publishResult PublishResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &publishResult); err != nil {
		return nil, err
	}

	return &publishResult, nil
}
