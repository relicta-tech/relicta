package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Transport handles the communication layer for MCP.
type Transport interface {
	// ReadMessage reads the next JSON-RPC message.
	ReadMessage() (*Request, error)
	// WriteResponse writes a JSON-RPC response.
	WriteResponse(resp *Response) error
	// WriteNotification writes a JSON-RPC notification.
	WriteNotification(method string, params any) error
	// Close closes the transport.
	Close() error
}

// StdioTransport implements Transport over stdin/stdout.
// Messages are newline-delimited JSON (NDJSON).
type StdioTransport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
	closed  bool
}

// NewStdioTransport creates a new stdio transport.
func NewStdioTransport(reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

// ReadMessage reads the next JSON-RPC request from stdin.
func (t *StdioTransport) ReadMessage() (*Request, error) {
	if t.closed {
		return nil, io.EOF
	}

	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	return &req, nil
}

// WriteResponse writes a JSON-RPC response to stdout.
func (t *StdioTransport) WriteResponse(resp *Response) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.closed {
		return io.ErrClosedPipe
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if _, err := t.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

// WriteNotification writes a JSON-RPC notification to stdout.
func (t *StdioTransport) WriteNotification(method string, params any) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.closed {
		return io.ErrClosedPipe
	}

	notification := Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
	}

	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal notification params: %w", err)
		}
		notification.Params = paramsData
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	if _, err := t.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// Close closes the transport.
func (t *StdioTransport) Close() error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	t.closed = true
	return nil
}

// WriteProgress sends a progress notification to stdout.
// This implements the ProgressWriter interface for MCP progress tracking.
func (t *StdioTransport) WriteProgress(notification *ProgressNotification) error {
	return t.WriteNotification("notifications/progress", notification)
}

// MessageLoop runs the main message processing loop.
type MessageLoop struct {
	transport Transport
	handler   MessageHandler
}

// MessageHandler processes incoming messages.
type MessageHandler interface {
	HandleRequest(ctx context.Context, req *Request) *Response
}

// NewMessageLoop creates a new message loop.
func NewMessageLoop(transport Transport, handler MessageHandler) *MessageLoop {
	return &MessageLoop{
		transport: transport,
		handler:   handler,
	}
}

// Run starts the message loop.
func (l *MessageLoop) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := l.transport.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// Send parse error response for malformed JSON
			resp := NewErrorResponse(nil, ErrCodeParseError, "Parse error", err.Error())
			_ = l.transport.WriteResponse(resp)
			continue
		}

		// Handle the request
		resp := l.handler.HandleRequest(ctx, req)
		if resp != nil {
			if err := l.transport.WriteResponse(resp); err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}
		}
	}
}
